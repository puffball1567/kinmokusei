import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import App from "./App";

interface Todo {
  id: number;
  title: string;
  completed: boolean;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function backendFetch(initialTodos: Todo[] = []) {
  let todos = initialTodos.map((todo) => ({ ...todo }));
  return vi.fn(async (input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
    const path = String(input);
    const framework = path.startsWith("/fiber-api") ? "Fiber" : "Gin";
    const method = options?.method ?? "GET";

    if (path.endsWith("/health")) {
      return jsonResponse({ status: "ok", language: "OnsenTamago", framework });
    }
    if (path.endsWith("/todos") && method === "GET") {
      return jsonResponse({ items: todos });
    }
    if (path.endsWith("/todos") && method === "POST") {
      const inputBody = JSON.parse(String(options?.body)) as { title: string };
      const todo = { id: Math.max(0, ...todos.map((item) => item.id)) + 1, title: inputBody.title.trim(), completed: false };
      todos = [...todos, todo];
      return jsonResponse({ item: todo }, 201);
    }
    const toggle = path.match(/\/todos\/(\d+)\/toggle$/);
    if (toggle && method === "PATCH") {
      const id = Number(toggle[1]);
      todos = todos.map((todo) => todo.id === id ? { ...todo, completed: !todo.completed } : todo);
      return jsonResponse({ item: todos.find((todo) => todo.id === id) });
    }
    const remove = path.match(/\/todos\/(\d+)$/);
    if (remove && method === "DELETE") {
      const id = Number(remove[1]);
      todos = todos.filter((todo) => todo.id !== id);
      return new Response(null, { status: 204 });
    }
    return jsonResponse({ error: "unexpected request" }, 500);
  });
}

function deferredResponse(signal?: AbortSignal | null) {
  let resolve!: (response: Response) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<Response>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
  return { promise, resolve };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("React web-framework demo", () => {
  it("loads Gin and exposes input boundaries without a page reload", async () => {
    const fetchMock = backendFetch([
      { id: 1, title: "Read the source", completed: true },
      { id: 2, title: "Ship the demo", completed: false },
    ]);
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(await screen.findByText("OnsenTamago is serving Gin on port 8080")).toBeVisible();
    expect(screen.getByText("1/2 complete")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "New todo title" })).toHaveAttribute("maxlength", "80");
    expect(screen.getByRole("button", { name: "Add todo" })).toBeDisabled();
    expect(fetchMock).toHaveBeenCalledWith("/gin-api/health", expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(fetchMock).toHaveBeenCalledWith("/gin-api/todos", expect.objectContaining({ signal: expect.any(AbortSignal) }));
  });

  it("creates, toggles, and deletes a todo through the selected backend", async () => {
    const user = userEvent.setup();
    const fetchMock = backendFetch([{ id: 1, title: "Existing", completed: false }]);
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByText("Existing");

    await user.type(screen.getByRole("textbox", { name: "New todo title" }), "  Test the UI  ");
    await user.click(screen.getByRole("button", { name: "Add todo" }));
    expect(await screen.findByText("Test the UI")).toBeVisible();
    expect(screen.getByText("0/2 complete")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Complete Test the UI" }));
    expect(await screen.findByRole("button", { name: "Mark Test the UI incomplete" })).toHaveTextContent("✓");
    expect(screen.getByText("1/2 complete")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Delete Test the UI" }));
    await waitFor(() => expect(screen.queryByText("Test the UI")).not.toBeInTheDocument());
    expect(screen.getByText("0/1 complete")).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith("/gin-api/todos", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenCalledWith("/gin-api/todos/2/toggle", expect.objectContaining({ method: "PATCH" }));
    expect(fetchMock).toHaveBeenCalledWith("/gin-api/todos/2", expect.objectContaining({ method: "DELETE" }));
  });

  it("keeps the current list visible until the new backend is ready", async () => {
    const user = userEvent.setup();
    const pending: ReturnType<typeof deferredResponse>[] = [];
    const fetchMock = vi.fn((input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      const path = String(input);
      if (path.startsWith("/gin-api")) {
        return Promise.resolve(path.endsWith("/health")
          ? jsonResponse({ status: "ok", language: "OnsenTamago", framework: "Gin" })
          : jsonResponse({ items: [{ id: 1, title: "Gin remains visible", completed: false }] }));
      }
      const request = deferredResponse(options?.signal);
      pending.push(request);
      return request.promise;
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByText("Gin remains visible");

    await user.click(screen.getByRole("button", { name: "Fiber backend" }));
    expect(await screen.findByText("Updating from Fiber…")).toBeVisible();
    expect(screen.getByText("Gin remains visible")).toBeVisible();
    expect(screen.queryByText("Loading Fiber…")).not.toBeInTheDocument();

    pending[0].resolve(jsonResponse({ status: "ok", language: "OnsenTamago", framework: "Fiber" }));
    pending[1].resolve(jsonResponse({ items: [{ id: 7, title: "Fiber is ready", completed: true }] }));
    expect(await screen.findByText("Fiber is ready")).toBeVisible();
    expect(screen.queryByText("Gin remains visible")).not.toBeInTheDocument();
    expect(screen.getByText("OnsenTamago is serving Fiber on port 8081")).toBeVisible();
  });

  it("aborts stale backend requests during rapid switching", async () => {
    const user = userEvent.setup();
    const fiberSignals: AbortSignal[] = [];
    let ginGeneration = 0;
    const fetchMock = vi.fn((input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      const path = String(input);
      if (path.startsWith("/fiber-api")) {
        if (options?.signal) fiberSignals.push(options.signal);
        return deferredResponse(options?.signal).promise;
      }
      if (path.endsWith("/health")) {
        ginGeneration++;
        return Promise.resolve(jsonResponse({ status: "ok", language: "OnsenTamago", framework: "Gin" }));
      }
      return Promise.resolve(jsonResponse({ items: [{ id: ginGeneration, title: `Gin generation ${ginGeneration}`, completed: false }] }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByText("Gin generation 1");

    await user.click(screen.getByRole("button", { name: "Fiber backend" }));
    await screen.findByText("Updating from Fiber…");
    await user.click(screen.getByRole("button", { name: "Gin backend" }));

    expect(await screen.findByText("Gin generation 2")).toBeVisible();
    expect(fiberSignals).toHaveLength(2);
    expect(fiberSignals.every((signal) => signal.aborted)).toBe(true);
    expect(screen.queryByText("Fiber is unavailable")).not.toBeInTheDocument();
  });

  it("shows a backend failure and recovers through Retry", async () => {
    const user = userEvent.setup();
    let failing = true;
    const healthyFetch = backendFetch([{ id: 1, title: "Recovered todo", completed: false }]);
    const fetchMock = vi.fn((input: RequestInfo | URL, options?: RequestInit) => {
      if (failing) return Promise.reject(new Error("connection refused"));
      return healthyFetch(input, options);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(await screen.findByRole("alert")).toHaveTextContent("Gin is unavailable: connection refused");
    failing = false;
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Recovered todo")).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("keeps mutation failures visible without corrupting the local list", async () => {
    const user = userEvent.setup();
    const healthyFetch = backendFetch();
    let failingMethod = "";
    const fetchMock = vi.fn((input: RequestInfo | URL, options?: RequestInit) => {
      const method = options?.method ?? "GET";
      if (method === failingMethod) {
        if (method === "PATCH") return Promise.resolve(new Response("not JSON", { status: 500 }));
        return Promise.resolve(jsonResponse({ error: `${method.toLowerCase()} failed` }, 409));
      }
      return healthyFetch(input, options);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    expect(await screen.findByText("Nothing here yet. Add the first todo.")).toBeVisible();

    failingMethod = "POST";
    await user.type(screen.getByRole("textbox", { name: "New todo title" }), "Durable item");
    await user.click(screen.getByRole("button", { name: "Add todo" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("post failed");
    expect(screen.queryByText("Durable item")).not.toBeInTheDocument();

    failingMethod = "";
    await user.click(screen.getByRole("button", { name: "Add todo" }));
    expect(await screen.findByText("Durable item")).toBeVisible();

    failingMethod = "PATCH";
    await user.click(screen.getByRole("button", { name: "Complete Durable item" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Request failed (500)");
    expect(screen.getByRole("button", { name: "Complete Durable item" })).toBeVisible();

    failingMethod = "DELETE";
    await user.click(screen.getByRole("button", { name: "Delete Durable item" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("delete failed");
    expect(screen.getByText("Durable item")).toBeVisible();
  });
});
