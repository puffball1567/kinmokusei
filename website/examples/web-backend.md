# React with Gin and Fiber

The repository includes one React and TypeScript frontend with interchangeable Gin and Fiber backends written in OnsenTamago.

Both backends:

- import the real external Go framework package;
- share an OnsenTamago todo store;
- expose health, list, create, toggle, and delete operations;
- compile to ordinary Go;
- run against the same HTTP contract as independent handwritten Go servers;
- are exercised through the Vite development proxy.

Browse the [complete example](https://github.com/puffball1567/onsentamago/tree/main/examples/react-web-frameworks) to compare the `.otm` backend with its generated behavior and independent Go oracle.
