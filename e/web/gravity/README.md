# Gravity Web UI Enterprise Edition

This package contains the proprietary code of Gravity Web UI.

## How it works

It references the open source Gravity code via npm dependency
```
  "dependencies": {
    "@gravitational/gravity": "^1.0.0",
      ...
  },
```

Which allows referencing the open source modules as regular npm modules.

```jsx
// oss imports
import Cluster from 'gravity/cluster/components';
import FeatureDashboard from 'gravity/cluster/features/featureDashboard';
import FeatureAccount from 'gravity/cluster/features/featureAccount';
import FeatureNodes from 'gravity/cluster/features/featureNodes';
import FeatureLogs from 'gravity/cluster/features/featureLogs';
```

Note that in the above example, we are using aliases to shorten the package name
`@gravitational/gravity` -> `gravity`.

See the `@gravitational/build` package where these aliases are defined.