# Teleport Web UI Enterprise Edition

This package contains the proprietary code of Teleport Web UI.

## How it works

It references the open source Teleport code via npm dependency.

```
  "dependencies": {
    "@gravitational/teleport": "^1.0.0",
      ...
  },
```

Which allows referencing the open source modules as regular npm module.

```jsx
import Teleport from 'teleport';
import cfg from 'teleport/config';
import EntepriseCluster from './cluster';

ReactDOM.render(
  <Teleport>
    <Route path={cfg.routes.cluster} component={EntepriseCluster} />
  </Teleport>,
  document.getElementById('app')
);
```
