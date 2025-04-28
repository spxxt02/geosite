#!/bin/bash

set -e -o pipefail

cd output
git init
git config --local user.email "github-action@users.noreply.github.com"
git config --local user.name "GitHub Action"
git remote add origin https://github-action:$GITHUB_TOKEN@github.com/lhear/geosite.git
git branch -M release
git add .
git commit -m "Update release"
git push -f origin release