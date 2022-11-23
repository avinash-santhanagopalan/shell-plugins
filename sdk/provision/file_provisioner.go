package provision

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/1Password/shell-plugins/sdk"
)

// FileProvisioner provisions one or more secrets as a temporary file.
type FileProvisioner struct {
	sdk.Provisioner

	fileContents        ItemToFileContents
	outfileName         string
	outpathFixed        string
	outpathEnvVar       string
	setOutpathAsArg     bool
	outpathPrefixedArgs []string
}

type ItemToFileContents func(in sdk.ProvisionInput) ([]byte, error)

// FieldAsFile can be used to store the value of a single field as a file.
func FieldAsFile(fieldName string) ItemToFileContents {
	return ItemToFileContents(func(in sdk.ProvisionInput) ([]byte, error) {
		if value, ok := in.ItemFields[fieldName]; ok {
			return []byte(value), nil
		} else {
			return nil, fmt.Errorf("no value present in the item for field '%s'", fieldName)
		}
	})
}

// TempFile returns a file provisioner and takes a function that maps a 1Password item to the contents of
// a single file.
func TempFile(fileContents ItemToFileContents, opts ...FileOption) sdk.Provisioner {
	p := FileProvisioner{
		fileContents: fileContents,
	}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

// FileOption can be used to influence the behavior of the file provisioner.
type FileOption func(*FileProvisioner)

// AtFixedPath can be used to tell the file provisioner to store the credential at a specific location, instead of
// an autogenerated temp dir. This is useful for executables that can only load credentials from a specific path.
func AtFixedPath(path string) FileOption {
	return func(p *FileProvisioner) {
		p.outpathFixed = path
	}
}

// Filename can be used to tell the file provisioner to store the credential with a specific name, instead of
// an autogenerated name. The specified filename will be appended to the path of the autogenerated temp dir.
// Gets ignored if the provision.AtFixedPath option is also set.
func Filename(name string) FileOption {
	return func(p *FileProvisioner) {
		p.outfileName = name
	}
}

// SetPathAsEnvVar can be used to provision the temporary file path as an environment variable.
func SetPathAsEnvVar(envVarName string) FileOption {
	return func(p *FileProvisioner) {
		p.outpathEnvVar = envVarName
	}
}

// SetPathAsArg can be used to provision the temporary file path as an arg that will be appended to
// the executable's command. The file path can be prefixed with the specified `prefixedArgs`. For example:
// `SetPathAsArg("--config-file")` will result in `--config-file /path/to/tempfile`.
func SetPathAsArg(prefixedArgs ...string) FileOption {
	return func(p *FileProvisioner) {
		p.setOutpathAsArg = true
		p.outpathPrefixedArgs = prefixedArgs
	}
}

func (p FileProvisioner) Provision(ctx context.Context, in sdk.ProvisionInput, out *sdk.ProvisionOutput) {
	contents, err := p.fileContents(in)
	if err != nil {
		out.AddError(err)
		return
	}

	outpath := ""
	if p.outpathFixed != "" {
		// Default to the provision.AtFixedPath option
		outpath = p.outpathFixed
	} else if p.outfileName != "" {
		// Fall back to the provision.Filename option
		outpath = in.FromTempDir(p.outfileName)
	} else {
		// If both are undefined, resort to generating a random filename
		outpath = in.FromTempDir(randomFilename())
	}

	out.AddSecretFile(outpath, contents)

	if p.outpathEnvVar != "" {
		// Populate the specified environment variable with the output path.
		out.AddEnvVar(p.outpathEnvVar, outpath)
	}

	if p.setOutpathAsArg {
		// Add args to specify the output path.
		out.AddArgs(p.outpathPrefixedArgs...)
		out.AddArgs(outpath)
	}
}

func (p FileProvisioner) Deprovision(ctx context.Context, in sdk.DeprovisionInput, out *sdk.DeprovisionOutput) {
	// Nothing to do here: deleting the files gets taken care of.
}

func (p FileProvisioner) Description() string {
	return "Provision secret file"
}

func randomFilename() string {
	rand.Seed(time.Now().UnixNano())
	length := 16
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}
