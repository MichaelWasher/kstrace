#!/bin/bash

# Run all other tests
bash e2e_smoke_tests/*.sh

python3 e2e_smoke_tests/*.py
