# Goals being worked on
- [ ] Increasing test coverage
- [ ] Server mode improvements
    - [ ] varlink API refinement
    - [ ] better logging
    - [ ] standardize config names (`update_webhook` -> `update_endpoint`)
    - [ ] Prometheus compatible logging option
- [ ] Component-level output directories/temporary directories
- [ ] Podman registry login support

# The Road to 1.0 (TBD)
- [ ] OpenBao as attrributes engine, or some equivalent
- [ ] Improve Service manifest definitions
- [ ] Remote components refinement
- [ ] Add rootless integration tests
- [ ] Public varlink API
- [ ] Component Namspacing
- [ ] `service` and other types of rollback settings
    - [ ] stabilization window
    - [ ] server mode retry limit?


# Other Potential future goals

- [ ] Some type of in-repo per-host systemd override management
- [ ] Smarter rollback
- [ ] Slimmer container image
- [ ] [X-Materia] option in Quadlet files to embed e.g. Service information in the resource instead of in the Manifest
- [ ] Better `.kube` handling
- [ ] Check containers in pod health on update
