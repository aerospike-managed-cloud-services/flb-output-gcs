# flb-output-gcs

Fluent-bit output plugin to write to GCS buckets 

## Installation

1. Download the [latest release]. You most likely want the file named like: `flb-output-gcs-vX.Y.Z_linux_amd64.tar.gz`

1. Unpack.

    ```
    tar xvfz flb-output-gcs-v*.tar.gz
    ```

1. Copy `./out_gcs.so` somewhere. You will use its location in the plugin config (see below).


### For contributors: Install and run tests

1. [Install Go](https://go.dev/doc/install)
1. Check out the source code with `git clone` from the [repo](https://github.com/aerospike-managed-cloud-services/flb-output-gcs/)
1. Install the dependencies and run tests:

  ```
  go install
  make test
  ```

### Enable the plugin and configure

Ref [Fluent-bit configuration](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/configuration-file)

Enable the plugin by 

1. Passing `-e .../path/to/out_gcs.so` on the command-line, _or_
2. Adding the plugin to `plugins.conf` with `Path .../path/to/out_gcs.so`

Then, create one or more `[OUTPUT]` sections in the top-level config file with `name gcs`. Example:

```
[OUTPUT]
    name gcs
    match cpu.local
    # set this to anything unique; must not be the same as any other [OUTPUT] block
    outputid cpu.local

    Bucket my-nifty-log-bucket
    BufferSizeKiB 1000
    Compression gzip
```

Plugin Options         |     |     |
---------------------- | --- | --- |
*Bucket*               | Name of the bucket where we'll store logs | required, no default
*BufferSizeKiB*        | Maximum size (in KiB) held in the request Writer buffer before committing an object to the bucket | default 5000
*BufferTimeoutSeconds* | Maximum time (in s) between writes before the requst Writer must commit to the bucket (even if bufferSizeKiB has not been reached) | default 300
*Compression*          | Compression type, allowed values: `none`; `gzip` | default `none`
*OutputID*             | String to uniquely identify this output plugin instance | required, no default
*ObjectNameTemplate*   | Template for the object filename that gets created in the bucket. (see below) | default `{{.InputTag}}-{{.Timestamp}}-{{.Uuid}}`

### ObjectNameTemplate syntax

The object name is constructed from Go [text/template] syntax. Any character that's valid in a bucket object name is permitted, including `/`.

The following placeholders are recognized:

- `{{ .InputTag }}` the tag of the associated fluent "input" being flushed, e.g. "cpu.local"
- `{{ .Timestamp }}` timestamp using unix seconds since 1970-01-01
- `{{ .IsoDateTime }}` 14-digit YYYYmmddTHHMMSSZ datetime format, UTC (ex.: `20220211T171643Z`)
- `{{ .Yyyy }}` year, `{{ .Mm }}` month, `{{ .Dd }}` day of month
- `{{ .BeginTime.Format "2006...." }}` .BeginTime is a [time.Time()] object and you can use any method on it; for example, you can call the `.Format` method, as shown, and get any format you want. [Go time Format reference]
- `{{ .Uuid }}` a random UUID

[text/template]: https://pkg.go.dev/text/template
[time.Time()]: https://pkg.go.dev/time#Time
[Go time Format reference]: https://pkg.go.dev/time#Time.Format

The object created from this name will be stored at `gs://<bucket>/<rendered_template>`

If `Compression gzip` is enabled, we also add `.gz` to the end of the bucket object name, as in `gs://<bucket>/<rendered_template>.gz`

## Google Credentials

To use a service account with the `gcs` plugin, set `GOOGLE_APPLICATION_CREDENTIALS` in the environment before running `fluent-bit`. [Google API reference](https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable)

Example:

```
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/my-svc-acct-key.json
/usr/local/bin/fluent-bit -e out_gcs.so
```

If this environment variable is unset, the plugin uses Google [Application Default Credentials](https://cloud.google.com/docs/authentication/production#automatically).

In a user's development environment, this is likely set with `gcloud config <...>`.

----

## Maintainer section: releasing

To cut a release of this software, automated tests must pass. Check under `Actions` for the latest commit.

#### Create an RC branch and test

- We use the Gitflow process. For a release, this means that you should have a v1.2.3-rc branch under your 
  develop branch. Like this:
  ```
    main  
    └── develop  
        └── v1.2.3-rc
  ```

- Update *this file* (README.md).
  
  1. Confirm that the docs make sense for the current release.
  1. Check links!
  1. Update the Changelog section at the bottom.

- Perform whatever tests are necessary.

#### Tag and cut the release with Github Actions

- Once you have tested in this branch, create a tag in the v1.2.3-rc branch:
  ```
  git tag -a -m v1.2.3 v1.2.3
  git push --tags
  ```

- Navigate to the [Github Actions URL] for this repo, and run the action labeled `publish release`.

    - You will be asked to choose a branch. Choose your rc branch, e.g. `v1.2.3-rc`

    - :warning: If you run this action without creating a tag on v1.2.3-rc first, the action will fail with an error and nothing will happen.

  If you have correctly tagged a commit and chosen the right branch, this will run and create a new release on the [Releases page].

- Edit the release on that page 

#### Merge up

- Finish up by merging your `-rc` branch into 
  1. `main` and then 
  2. `develop`.


## Changelog

<details><summary>(About: Keep-a-Changelog text format)</summary>

The format is based on [Keep a Changelog], and this project adheres to [Semantic
Versioning].
</details>

### [0.2.2]

- Fix for http2/rapid-reset CVE-2023-44487

### [0.2.1]

- Unknown

### [0.2.0]

#### Added

- Robust unit test coverage and automated CI
- Uuid template for object name

#### Fixed

- Byte arrays read from fluent-bit are interpreted as utf-8 strings and logged as strings

#### Changed

- Calls to fluent-bit API and GCS API are now made through interfaces to make them testable
- Switch to structured logging
- Use JSON marshalling to produce the file

### [0.1.0]

- Non-public interim release; no tests yet.

#### Added

- Basic plugin and all plugin options; see README

### [0.0]

- Brand-new repo.


[Unreleased]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/compare/v0.2.2..HEAD

[0.2.2]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/compare/v0.2.1..v0.2.2
[0.2.1]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/compare/v0.2.0..v0.2.1
[0.2.0]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/compare/v0.1.0..v0.2.0
[0.1.0]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/compare/v0.0..v0.1.0
[0.0]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/tree/v0.0


[latest release]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/releases/latest

[Github Actions URL]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/actions

[Releases page]: https://github.com/aerospike-managed-cloud-services/flb-output-gcs/releases

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/

[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
