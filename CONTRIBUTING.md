# CONTRIBUTING

Thank you for your interest in contributing! Help is always appreciated, whether through bugfixes, feature suggestions/implementations, or just general testing and usage of the project.

Note that as this is a new and relatively small project these policies may change.

## Bugs, issues, ideas, questions?

- Any type of discussions is allowed in the [Matrix room](https://matrix.to/#/#materia:saintnet.tech).
- Bug reports and feature requests can either be brought up in the Matrix room or as a Github Discussion. Please do not create a Github Issue directly.
- If you are an existing contributor, bug reports can be filed directly using `git-bug`.

## AI Policy
Submissions using LLMs for the submitted code must indicate as such, preferably using the `Assisted-by` header as documented in the [Fedora AI-Assisted Contributions Policy](https://docs.fedoraproject.org/en-US/council/policy/ai-contribution-policy/). They will be evaluated on a case-by-case basis. Small contributions or minor documentation updates are more likely to be allowed than large contributions.

Merge request, feature requests, and bug reports must be created by humans. Automatically generated issue reports or merge requests will not be accepted. Violators will be banned.

## Contributing and testing code

First off, thanks for even getting this far!

It's recommended to use [mise](https://mise.jdx.dev/) when developing. The materia project uses mise to manage all external dependencies and tasks for building and testing the project.

Basic contribution overview:

1. Build your changes with `mise r build`
2. Lint and run unit tests with `mist r test`
3. If possible, run the integration tests with `mise r virter-test`
4. Submit your merge request or patchset.

### Building and testing the project.

1. `mise r build` will generate `amd64` and `arm64` binaries in the `./bin` folder. Your changes must work for both platforms to be accepted

2. Materia uses `golangci-lint` for linting and formatting. It is recommended to run this before committing any changes; if your editor does not automatically do so, you can manually run `mise r fmt` and `mise r lint`. You can also run `golangci-lint` directly.

3. `mise r test` will run the formatter, linter, and unit tests for the project. All tests should pass.

4. Materia has a series of integrations tests in `./virter/integration_test.go` that are designed to be run in a VM. The officially supported and automated way of doing this is with the [virter](https://github.com/LINBIT/virter) tool and the `mise r virter-test` task. If you are having issues setting up virter please see the [setting up virter](#setting-up-virter) section. `virter-test` will automatically build materia, create the virter VM, copy the latest build and test files into the vm, and run `mise r virter-testsuite`. If you are just making a small code change you do not need to do this, however these tests will be run on any merge requests or patches submitted before acceptance.

5. Submit your changes with descriptive commit message. The [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) standard is recommended.

### Submitting your changes

Right now the easiest way to submit your changes is through a Github pull request, however we are also happy to accept emailed patches or otherwise contributed patchsets.

If your changes are contributing a new feature, please include documentation and test cases for said feature. If you are unsure or otherwise unable to do so, please let us know in your PR so a maintainer can write it at their leisure. PRs without documentation and test cases **will not be accepted** until such are added.

If you plan on contributing more to your PR before review but after opening it, please mark it as a draft.


## Setting up virter

Virter is a tool for creating libvirt VMs quickly and automatically. It will be auto-installed by mise when you run the `mise r virter-test` command. Otherwise, you can install it directly from the [virter repository](https://github.com/LINBIT/virter).

If you have not used libvirt virtual machines before, you may need to set it up and create a `default` storage pool.

Virter will create a config file in `$HOME/.config/virter/virter.toml` on first run. If you are using a newer Linux distro, you may need to update this file to have the correct libvirt socket configured.

```
# Default value: socket = "/var/run/libvirt/libvirt-sock"
# Newer distro value: socket = "/var/run/libvirt/virtqemud-sock"
```

This is if your distro implements the new multi-service version of libvirt.

If you can run the following commands from the virter README successfully, you should be ready to run `mise r virter-test`:

```bash
virter image pull alma-8 # also would be auto-pulled in next step
virter vm run --name alma-8-hello --id 100 --wait-ssh alma-8
virter vm ssh alma-8-hello
virter vm rm alma-8-hello
```
