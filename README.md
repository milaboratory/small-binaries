# Repository structure

Each go file = single binary. 
This repo intentionally has no go.mod file as it is designed
for small binary utilities with small implementation.

The binary version is detected from `version` in package.json file

All binaries have their software descriptors inside NPM package released along with the packages.
Say, for `sleep` command, there is a `@milaboratory/small-binaries:sleep` software descirptor to be used in
Platforma workflows

## Patching existing binaries
* Change the code
* Check it can be built: `go build -o /dev/null ./read-file-to-stdout-with-sleep.go`
* Bump 2 package versions in `package.json`: the version of NPM package and the version of software package
  (patch or minor, depending on the changes you made)
* Commit and push changes.
  All updates to the `main` branch with version change in `package.json` are released to registry and NPM

## Adding new binaries
* Create `.go` file in root directory
* Write the code.
* Add new file name (without `.go` extension) to `./scripts/build.sh` and `./scripts/publish.sh`
* Add new software package into `package.json` (`block-software.packages.<file name>`).
* Bump package version in `package.json` (minor version, as you added new binary)
* Commit and push changes.
