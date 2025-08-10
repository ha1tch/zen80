#!/bin/bash

rompath=$(realpath "./rom/128-0.rom")

export Z80_ROM_PATH="$rompath"
export Z80_ROM_STEPS="50000000"

go test ./z80 -v -count=1
