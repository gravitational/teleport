// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

#include "client_darwin.h"

void BundlePath(struct BundlePathResult *result) {
    // From the docs:
    // > This method may return a valid bundle object even for unbundled apps.
    // > It may also return nil if the bundle object could not be created,
    // > so always check the return value. 
    NSBundle *main = [NSBundle mainBundle];
    if (!main) {
        result->bundlePath = strdup("");
        return;
    }
    
    result->bundlePath = VNECopyNSString([main bundlePath]);
}

char *VNECopyNSString(NSString *val) {
    if (val) {
        return strdup([val UTF8String]);
    }
    return strdup("");
}
