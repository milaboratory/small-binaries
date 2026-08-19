---
"@platforma-open/milaboratories.software-test-utils.python-stub": patch
"@platforma-open/milaboratories.software-test-utils.java-stub": patch
"@platforma-open/milaboratories.software-test-utils": patch
---

Add a `docker` entrypoint to the python and java stub packages, so tests that
run them work on runners without local binary execution (Kubernetes).

The images run real python and real java. They print the same command line
report as the fake run environments of the binary distribution, byte for byte,
so the output contract of the consuming tests does not change.
