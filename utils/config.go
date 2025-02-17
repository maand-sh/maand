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

func GetMaandConf() MaandConf {
	maandConf := path.Join(bucket.Location, "maand.conf")
	if _, err := os.Stat(maandConf); err == nil {
		maandData, err := os.ReadFile(maandConf)
		Check(err)

		var maandConf MaandConf
		err = toml.Unmarshal(maandData, &maandConf)
		Check(err)

		if maandConf.CertsTTL == 0 {
			maandConf.CertsTTL = 60
		}

		return maandConf
	}
	return MaandConf{}
}

func WriteMaandConf(conf *MaandConf) {
	data, err := toml.Marshal(conf)
	Check(err)
	maandConfPath := path.Join(bucket.Location, "maand.conf")
	if _, err := os.Stat(maandConfPath); os.IsNotExist(err) {
		err = os.WriteFile(maandConfPath, data, os.ModePerm)
		Check(err)
	}
}
