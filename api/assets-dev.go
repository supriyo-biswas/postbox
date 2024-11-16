//go:build !release

package api

import "embed"

const useEmbed = false

var embedFiles embed.FS
