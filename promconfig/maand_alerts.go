// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import _ "embed"

//go:embed maand_certs_alerts.yaml
var MaandCertAlertsYAML []byte

const MaandAlertsJob = "maand"

const MaandCertAlertsFile = "certs.yaml"
