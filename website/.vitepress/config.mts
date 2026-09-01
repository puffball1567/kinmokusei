import { defineConfig } from "vitepress";

export default defineConfig({
  title: "OnsenTamago",
  description: "TypeScript-inspired syntax that compiles to readable Go",
  base: "/onsentamago/",
  cleanUrls: true,
  lastUpdated: true,
  sitemap: {
    hostname: "https://puffball1567.github.io/onsentamago/",
  },
  head: [
    ["meta", { name: "theme-color", content: "#d97745" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:title", content: "OnsenTamago" }],
    [
      "meta",
      {
        property: "og:description",
        content: "Write TypeScript-inspired source and compile it to readable Go.",
      },
    ],
  ],
  themeConfig: {
    siteTitle: "OnsenTamago",
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Language", link: "/guide/language-basics" },
      { text: "Reference", link: "/reference/cli" },
      { text: "Examples", link: "/examples/web-backend" },
      {
        text: "v0.2",
        items: [
          { text: "v0.1.0 release", link: "https://github.com/puffball1567/onsentamago/releases/tag/v0.1.0" },
          { text: "All releases", link: "https://github.com/puffball1567/onsentamago/releases" },
        ],
      },
    ],
    sidebar: [
      {
        text: "Start here",
        items: [
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "Language basics", link: "/guide/language-basics" },
          { text: "Types and data", link: "/guide/types-and-data" },
          { text: "Classes and structs", link: "/guide/classes-and-structs" },
          { text: "Errors and nullability", link: "/guide/errors-and-nullability" },
          { text: "Concurrency", link: "/guide/concurrency" },
          { text: "Go interoperability", link: "/guide/go-interop" },
          { text: "C FFI", link: "/guide/c-ffi" },
        ],
      },
      {
        text: "Build applications",
        items: [
          { text: "Projects and CLI", link: "/guide/projects-and-cli" },
          { text: "Editor setup", link: "/guide/editor" },
          { text: "React + Go backends", link: "/examples/web-backend" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI commands", link: "/reference/cli" },
          { text: "Implementation status", link: "/reference/status" },
        ],
      },
    ],
    socialLinks: [
      { icon: "github", link: "https://github.com/puffball1567/onsentamago" },
    ],
    search: {
      provider: "local",
    },
    editLink: {
      pattern: "https://github.com/puffball1567/onsentamago/edit/devel/website/:path",
      text: "Edit this page on GitHub",
    },
    footer: {
      message: "Released under the Apache License 2.0.",
      copyright: "OnsenTamago contributors",
    },
  },
});
