import { useCallback, useEffect, useState, type FormEvent } from "react";

type BackendKey = "gin" | "fiber";

interface Backend {
  label: string;
  endpoint: string;
  port: number;
  description: string;
}

interface Health {
  status: string;
  language: string;
  framework: string;
}

interface Todo {
  id: number;
  title: string;
  completed: boolean;
}

const backends: Record<BackendKey, Backend> = {
  gin: { label: "Gin", endpoint: "/gin-api", port: 8080, description: "net/http ecosystem" },
  fiber: { label: "Fiber", endpoint: "/fiber-api", port: 8081, description: "Express-inspired API" },
};

async function apiRequest<T>(backend: BackendKey, path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${backends[backend].endpoint}${path}`, {
    headers: options?.body ? { "Content-Type": "application/json" } : undefined,
    ...options,
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({})) as { error?: string };
  if (!response.ok) throw new Error(body.error ?? `Request failed (${response.status})`);
  return body as T;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown request error";
}

export default function App() {
  const [backend, setBackend] = useState<BackendKey>("gin");
  const [health, setHealth] = useState<Health | null>(null);
  const [todos, setTodos] = useState<Todo[]>([]);
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const [healthResult, todoResult] = await Promise.all([
        apiRequest<Health>(backend, "/health", { signal }),
        apiRequest<{ items: Todo[] }>(backend, "/todos", { signal }),
      ]);
      setHealth(healthResult);
      setTodos(todoResult.items);
    } catch (requestError) {
      if (signal?.aborted) return;
      setHealth(null);
      setTodos([]);
      setError(`${backends[backend].label} is unavailable: ${errorMessage(requestError)}`);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [backend]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function createTodo(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!title.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const result = await apiRequest<{ item: Todo }>(backend, "/todos", {
        method: "POST",
        body: JSON.stringify({ title }),
      });
      setTodos((current) => [...current, result.item]);
      setTitle("");
    } catch (requestError) {
      setError(errorMessage(requestError));
    } finally {
      setSaving(false);
    }
  }

  async function toggleTodo(id: number) {
    setError("");
    try {
      const result = await apiRequest<{ item: Todo }>(backend, `/todos/${id}/toggle`, { method: "PATCH" });
      setTodos((current) => current.map((todo) => (todo.id === id ? result.item : todo)));
    } catch (requestError) {
      setError(errorMessage(requestError));
    }
  }

  async function deleteTodo(id: number) {
    setError("");
    try {
      await apiRequest<void>(backend, `/todos/${id}`, { method: "DELETE" });
      setTodos((current) => current.filter((todo) => todo.id !== id));
    } catch (requestError) {
      setError(errorMessage(requestError));
    }
  }

  const completed = todos.filter((todo) => todo.completed).length;
  const selected = backends[backend];
  const connected = !loading && health?.framework === selected.label;

  return (
    <main className="shell">
      <section className="hero">
        <div className="eyebrow">React × OnsenTamago × Go</div>
        <h1>One app. Two Go frameworks.</h1>
        <p className="intro">
          The same OnsenTamago API runs through Gin and Fiber. Switch backends while keeping the
          React application and HTTP contract unchanged.
        </p>

        <div className="backend-switch" role="group" aria-label="Backend framework">
          {(Object.entries(backends) as [BackendKey, Backend][]).map(([key, item]) => (
            <button
              aria-label={`${item.label} backend`}
              className={backend === key ? "backend-option active" : "backend-option"}
              key={key}
              onClick={() => setBackend(key)}
              type="button"
            >
              <strong>{item.label}</strong>
              <span>{item.description}</span>
            </button>
          ))}
        </div>

        <div className={connected ? "connection connected" : loading ? "connection loading" : "connection"}>
          <span className="status-dot" aria-hidden="true" />
          {loading
            ? `${health ? "Switching" : "Connecting"} to ${selected.label} on port ${selected.port}`
            : connected
            ? `${health.language} is serving ${health.framework} on port ${selected.port}`
            : `Waiting for ${selected.label} on port ${selected.port}`}
        </div>
      </section>

      <section className="workspace" aria-busy={loading}>
        <div className="workspace-heading">
          <div>
            <p className="section-label">Live API state</p>
            <h2>Ship the demo</h2>
          </div>
          <span className="counter">{completed}/{todos.length} complete</span>
        </div>

        <form className="todo-form" onSubmit={createTodo}>
          <label className="sr-only" htmlFor="todo-title">New todo title</label>
          <input
            disabled={loading}
            id="todo-title"
            maxLength={80}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="Add something worth shipping…"
            value={title}
          />
          <button disabled={loading || saving || !title.trim()} type="submit">
            {saving ? "Adding…" : "Add todo"}
          </button>
        </form>

        {error && (
          <div className="error" role="alert">
            <span>{error}</span>
            <button onClick={() => void load()} type="button">Retry</button>
          </div>
        )}

        <div className={loading && todos.length > 0 ? "todo-content refreshing" : "todo-content"}>
          {loading && todos.length > 0 && (
            <div className="refresh-label" role="status">Updating from {selected.label}…</div>
          )}
          {loading && todos.length === 0 ? (
            <div className="empty">Loading {selected.label}…</div>
          ) : todos.length === 0 && !error ? (
            <div className="empty">Nothing here yet. Add the first todo.</div>
          ) : (
            <ul className="todo-list">
              {todos.map((todo) => (
                <li className={todo.completed ? "todo completed" : "todo"} key={todo.id}>
                  <button
                    aria-label={todo.completed ? `Mark ${todo.title} incomplete` : `Complete ${todo.title}`}
                    className="check"
                    disabled={loading}
                    onClick={() => void toggleTodo(todo.id)}
                    type="button"
                  >
                    {todo.completed ? "✓" : ""}
                  </button>
                  <span>{todo.title}</span>
                  <button
                    aria-label={`Delete ${todo.title}`}
                    className="delete"
                    disabled={loading}
                    onClick={() => void deleteTodo(todo.id)}
                    type="button"
                  >
                    Delete
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    </main>
  );
}
