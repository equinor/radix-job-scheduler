# Radix job scheduler
The job scheduler for application jobs

## Usage
Use [Radix job scheduler server](https://github.com/equinor/radix-job-scheduler-server) or this component as a reference to work with scheduled jobs.

## Developing

You need Go installed. Make sure `GOPATH` and `GOROOT` are properly set up.

Clone the repo into your `GOPATH` and run `go mod download`.

### Generating mocks
We use gomock to generate mocks used in unit test. [https://github.com/golang/mock](https://github.com/golang/mock)

You need to regenerate mocks if you make changes to any of the interfaces in the code, e.g. the job Handler interface

Run `make generate-mock` to regenerate mocks

#### Update version
We follow the [semantic version](https://semver.org/) as recommended by [go](https://blog.golang.org/publishing-go-modules).
* `tag` in git repository (in `main` branch)

  Run following command to set `tag` (with corresponding version)
    ```
    git tag v1.0.0
    git push origin v1.0.0
    ```
