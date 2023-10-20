/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

// teleportProdOCIPubKey is the key used to sign Teleport distroless images.
// The key lives in the Teleport production AWS KMS.
// In case of controlled rotation, we will want to add a second validator with
// the new key to support the transition period.
var teleportProdOCIPubKey = []byte(`-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAx+9UZboMl9ibwu/IWqbX
+wEJeKJqVpaLEsy1ODRpzIgcgaMh2n3BWtFEIoEszR3ZNlGdfqoPmb0nNnWx/qSf
eEsoSXievXa63M/gAUBB+jecbGEJH+SNaJPMVuvjabPqKtoMT2Spw3cacqpINzq1
rkWU8IawY333gXbwzgsuK7izT7ymgOLPO9qPuX7Q3EBaGw3EvY7u6UKtqhvSGdyr
MirEErOERQ8EP8TrkCcJk0UfPAukzIcj91uHlXaqYBD/IyNYiC70EOlSLoN5/EeA
I4jQnGRfaKF6H6K+WieX9tP9k8/02S+1EVJW592pdQZhJZEq1B/dMc8UR3IjPMMC
qCT2xT6TsinaVzDaAbaRf0hvp311GxwrckNofGm/OSLn1+HqM6q4/A7qHubeRXGO
byabRr93CHSLegZ7OBMswHqqnu6/DuXjc6gOsQkH09dVTFeh34rQy4GKrvnpmOwj
Er1ccxzKcF/pw+lxi07hkpihR/uHUPxFboA/Wl7H2Jub21MFwIFQrDJv7z8yQgxJ
EuIXJJox2oAL7NzdSi9VIUYnEnx+2EtkU/spAFRR6i1BnT6aoIy3521B76wnmRr9
atCSKjt6MdRxgj4htCjBWWJAGM9Z/avF4CYFmK7qiVxgpdrSM8Esbt2Ta+Lu3QMJ
T8LjqFu3u3dxVOo9RuLk+BkCAwEAAQ==
-----END PUBLIC KEY-----`)
