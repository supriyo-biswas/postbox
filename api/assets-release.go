//go:build release

package api

import "embed"

const useEmbed = true

//go:embed dist
var embedFiles embed.FS
