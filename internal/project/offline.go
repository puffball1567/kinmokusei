package project

// OfflineEnvironment closes proxy, direct VCS, checksum service and automatic
// toolchain download paths. GOPROXY=off alone is insufficient for private modules.
func OfflineEnvironment(environment []string) []string {
	for key, value := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": "none", "GOVCS": "*:off", "GOTOOLCHAIN": "local", "GOWORK": "off"} {
		environment = environmentWith(environment, key, value)
	}
	return environment
}
