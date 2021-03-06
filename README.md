# symutils
symutils is a collection of tools and libraries for managing symlinks, implemented in [Go Programming Language](http://golang.org).
This one can became real handy if you're "tag"ging your files by means of [creating symlinks under category-directories](http://freeconsole.org/anime/wiki/doku.php?id=articles:a_way_of_tagging_files). Or just happen to have dozens of troublesome symlinks around.

`replsym(1)` finds symlinks pointing to a target, or targets described by a pattern, and replaces them with a given, new target.

`lssym(1)` looks for (common) symlinks under given directories

`symfix(1)` finds and (somewhat interactively) repairs broken symlinks.

`xlocate(1)` is an alternative to locate. Common options are (mostly) compatible with GNU locate.

`dups(1)` finds duplicate files with the same name, using a locate database. Can remove the dups, or convert them to links pointing to a chosen "origin" file.

# Installation
You can install the tools using the go command. To install symfix for example, you can run

`go get github.com/salviati/symutils/symfix`

# Notes
cmd/* needs revision in error/warning reporting.

# Bugs
Reports bugs to bug@freeconsole.org.

# License
Published under GPL3. See the file named LICENSE for details.
