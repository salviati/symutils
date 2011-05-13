mk() {
    d=$PWD
    cd "$1"
    echo --- Making in "$1"
    gomake $2 || exit 1
    cd "$d"
}

PKG="fuzzy locate"
CMD="lssym replsym symfix xlocate"

targ="$@"
if [ -z "$targ" ]; then
    targ="install"
fi

for pkg in $PKG; do
    mk pkg/$pkg "$targ"
done

for cmd in $CMD; do
    mk cmd/$cmd "$targ"
done
