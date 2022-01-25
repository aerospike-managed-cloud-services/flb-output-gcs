# flb-output-gcs

Fluent-bit output plugin to write to GCS buckets 

## Installation

(replace)

1. steps to download

1. unpack

1. copy files


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

- Update *this file*.
  
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

- Navigate to ~~github actions URL for this repo~~ and run the action labeled `... release`.

    - You will be asked to choose a branch. Choose your rc branch, e.g. `v1.2.3-rc`

    - If you run this action without creating a tag on v1.2.3-rc first, the action will fail with an error and nothing will happen.

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

### versions [x.y.z] (replace)

- with changes listed; you should read [Keep a Changelog]


[Unreleased]: ~~url for ...HEAD~~

[x.y.z]: ~~url for v0.0..x.y.z~~

[0.0]: ~~url for the v0.0 tag~~


[latest release]: ~~url for /releases/latest~~

[Releases page]: ~~url for /releases~~

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/

[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
