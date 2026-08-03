#!/bin/sh
# labdash lives in two places: GitHub is home, because it has the wider reach.
# GitLab is the mirror, because labdash is a tool for GitLab.
#
# Git can push one remote to several URLs at once, so `git push` keeps both in
# sync on its own. That config lives in .git/config, which is not checked in and
# therefore missing on every fresh clone. This hook restores it.
#
# Wired up as a lefthook pre-push job. Note that Git runs pre-push once per push
# URL, so keep this fast and idempotent.

set -eu

# GITHUB_URL="git@github.com:giancarlosisasi/labdash.git"
# GITLAB_URL="https://gitlab.com/labdash/labdash.git"

# # Only touch the canonical repo. A fork has its own origin and must keep it.
# origin_url="$(git config --get remote.origin.url || true)"
# case "$origin_url" in
# 	*github.com[:/]giancarlosisasi/labdash*) ;;
# 	*gitlab.com[:/]labdash/labdash*) ;;
# 	*) exit 0 ;;
# esac

# has_push_url() {
# 	git config --get-all remote.origin.pushurl 2>/dev/null | grep -Fxq "$1"
# }

# if has_push_url "$GITHUB_URL" && has_push_url "$GITLAB_URL"; then
# 	exit 0
# fi

# # Adding a push URL now cannot change the push already in flight, because Git
# # resolved the URL list before it called this hook. So repair and stop.
# git config --unset-all remote.origin.pushurl || true
# git remote set-url --add --push origin "$GITHUB_URL"
# git remote set-url --add --push origin "$GITLAB_URL"

# echo "labdash: origin did not push to both GitHub and GitLab. Fixed:"
# git config --get-all remote.origin.pushurl | sed 's/^/labdash:   /'
# echo "labdash: run your push again to reach both remotes."
# exit 1
