#!/bin/bash

# Run all other tests
pytest -n 5 --log-cli-level=10 -vvv ./smoke_tests
