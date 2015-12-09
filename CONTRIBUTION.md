Contribution Guide
==================

Installation for Developer
--------------------------

Follow the instructions listed in the [README](README.md) file but instead of
running the `go get` command clone the repository using
```shell
git clone DEST TARGET
```

Now run
```shell
mkdir -p "${GOPATH}/github.com/AnalysisBotsPlatform"
ln -s TARGET "${GOPATH}/github.com/AnalysisBotsPlatform/platform"
```
where you replace `TARGET` by the directory in which you have cloned the
repository.


Bug reports & Feature requests
------------------------------

https://se.st.cs.uni-saarland.de/projects/analysis-bots-platform-for-github
