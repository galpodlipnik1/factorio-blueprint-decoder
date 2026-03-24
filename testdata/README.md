This directory holds the large Factorio fixture used by integration tests.

It is intentionally a nested Go module so the root library module published through `go get`
does not ship `blueprint-storage-2.dat`.

Run the integration tests from the repository root with:

`go test -tags=integration ./...`
