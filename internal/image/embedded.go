package image

import (
	_ "embed"
)

//go:embed embedded/coi_build.sh
var embeddedCoiBuildScript []byte

//go:embed embedded/dummy
var embeddedDummy []byte
