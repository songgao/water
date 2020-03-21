// +build !linux,!darwin,!windows,!freebsd,!netbsd,!openbsd

package water

// PlatformSpeficParams
type PlatformSpecificParams struct {
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{}
}
