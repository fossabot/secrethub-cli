package secrethub

import "os"

const (
	// defaultProfileDirName is the default name for the secrethub profile directory.
	defaultProfileDirName = ".secrethub"
	// defaultCredentialFilename is the name of the credential file.
	defaultCredentialFilename = "credential"
	// defaultCredentialFileMode is the filemode to assign to the credential file.
	defaultCredentialFileMode = os.FileMode(0600)
	// defaultProfileDirFileMode is the filemode to assign to the configuration directory.
	defaultProfileDirFileMode = os.FileMode(0700)

	// oldConfigFilename defines the filename for the file containing old configuration options.
	oldConfigFilename = "config"
)
