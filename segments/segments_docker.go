//go:build container

package segments

func init() {
	// This ensures a binary build by our Containerfile needs no configuration
	// changes to accomodate for a containers volume mountpoint.
	ContainerVolumePrefix = "/config/"
}
