#!/bin/bash

if [ ! -d ${HOME}/.sdkman ]; then
    echo "Install SDKMAN: curl -s \"https://get.sdkman.io?rcupdate=false\" | bash"
    echo "   then install Java SDK and Maven"
    exit 1
fi
source "${HOME}/.sdkman/bin/sdkman-init.sh"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if [ ! -d "${HOME}/epubcheck" ]; then
    echo "Unpack latest epubcheck into ${HOME}/epubcheck"
    exit 1
fi

java -jar "${HOME}/epubcheck/epubcheck.jar" -p default "$@"
