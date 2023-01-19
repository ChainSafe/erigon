## StreamingFast Firehose Fork of `Ethereum` (`erigon` client)

This is our Firehose instrumented fork of [ledgerwatch/erigon](https://github.com/ledgerwatch/erigon) repository. In this README, you will find instructions about how to work with this repository.

Read [Branches & Workflow](#branches-&-workflow) section for more details about how we handle branching model and versions.

### Initialization

The tooling and other instructions expect the following project
structure, it's easier to work with the Firehose fork when you use
the same names and settings.
```
cd ~/work
git clone --branch="firehose-v2" git@github.com:ChainSafe/go-erigon.git
cd go-erigon

git remote rename origin sf

git remote add origin https://github.com/ledgerwatch/erigon.git

git fetch origin
```

##### Assumptions

For the best result when working with this repository and the scripts it contains:

- The remote `sf` exists on main module and points to `git@github.com:ChainSafe/go-erigon.git`
- The remote `origin` exists on main module and points to https://github.com/ledgerwatch/erigon.git
