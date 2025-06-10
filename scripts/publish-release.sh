#!/bin/bash
set -eo pipefail

gh auth status

# pull latest tags from remote
LATEST_TAG=$(gh release list -L 1 |awk {'print $1'})

echo "=> fetching tags from remote ..."
git fetch origin
echo ""

echo "=> Here are the last 10 releases from github"
gh release list -L 10

read -rep $'\nWhich version do you like to release?\n=> ' GIT_TAG

tagRelease(){
  git tag $GIT_TAG
  git push origin $GIT_TAG
}

read -rep $'=> Do you with to create this tag / release?\n(y/n) => ' choice
case "$choice" in
  y|Y ) tagRelease;;
  n|N ) echo -e "\naborting ..."; exit 0;;
  * ) echo "invalid choice";;
esac
