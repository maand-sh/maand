// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"github.com/pelletier/go-toml/v2"
	"maand/bucket"

	"os"
	"path"
)

type MaandConf struct {
	UseSUDO            bool   `toml:"use_sudo"`
	SSHUser            string `toml:"ssh_user"`
	SSHKeyFile         string `toml:"ssh_key"`
	CertsTTL           int    `toml:"certs_ttl"`
	CertsRenewalBuffer int    `toml:"certs_renewal_buffer"`
}

func GetMaandConf() (MaandConf, error) {
	maandConf := path.Join(bucket.Location, "maand.conf")
	if _, err := os.Stat(maandConf); err == nil {
		maandData, err := os.ReadFile(maandConf)
		if err != nil {
			return MaandConf{}, err
		}

		var maandConf MaandConf
		err = toml.Unmarshal(maandData, &maandConf)
		if err != nil {
			return MaandConf{}, err
		}

		if maandConf.CertsTTL == 0 {
			maandConf.CertsTTL = 60
		}

		return maandConf, nil
	}
	return MaandConf{}, nil
}

func WriteMaandConf(conf *MaandConf) error {
	data, err := toml.Marshal(conf)
	if err != nil {
		return err
	}

	confPath := path.Join(bucket.Location, "maand.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		err = os.WriteFile(confPath, data, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
