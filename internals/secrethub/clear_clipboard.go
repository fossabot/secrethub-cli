package secrethub

import (
	"encoding/hex"
	"github.com/secrethub/secrethub-cli/internals/cli/clip"
	"github.com/secrethub/secrethub-cli/internals/cli/cloneproc"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// WriteClipboardAutoClear writes data to the clipboard and clears it after the timeout.
func WriteClipboardAutoClear(data []byte, timeout time.Duration, clipper clip.Clipper) error {
	hash, err := bcrypt.GenerateFromPassword(data, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = clipper.WriteAll(data)
	if err != nil {
		return err
	}

	err = cloneproc.Spawn(
		"clipboard-clear", hex.EncodeToString(hash),
		"--timeout", timeout.String())

	return err
}
