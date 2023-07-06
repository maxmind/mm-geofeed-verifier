# Releasing

* Install `goreleaser`. Refer to its docs.
* Set a `GITHUB_TOKEN` environment variable. Refer to `goreleaser` docs for
  information.
* Update `CHANGELOG.md`.
  * Mention recent changes.
  * Set a version if there is not one.
  * Set a release date.
* Commit `CHANGELOG.md`.
* Tag the release: `git tag -a v1.2.3 -m 'Tag v1.2.3'`.
* Push the tag: `git push origin v1.2.3`.
* Run `goreleaser`

NOTE: if a major version release is happening, it's necessary to update the `go.mod` file, as well as the import of internal packages according to the new major version (see more on [releasing modules v2 or higher](https://github.com/golang/go/wiki/Modules#releasing-modules-v2-or-higher))