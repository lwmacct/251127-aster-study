__main() {

  cd /apps/data/workspace/251118-fork-aster || exit 1
  git reset --hard HEAD && git clean -fd
  git checkout main
  git pull --rebase upstream main
  git push origin main

  ts=$(date "+%y%m%d-%H-%M")
  git checkout -b date/$ts
}

__main
