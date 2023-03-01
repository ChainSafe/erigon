## StreamingFast Firehose Fork of `Ethereum` (`erigon` client)

This is our Firehose instrumented fork of [ledgerwatch/erigon](https://github.com/ledgerwatch/erigon) repository. In this README, you will find instructions about how to work with this repository.

### Initialization

The tooling and other instructions expect the following project
structure, it's easier to work with the Firehose fork when you use
the same names and settings.
```
cd ~/work
git clone --branch="firehose-v2" git@github.com:ChainSafe/erigon.git
cd erigon

git remote rename origin sf

git remote add origin https://github.com/ledgerwatch/erigon.git

git fetch origin
```

### Assumptions

For the best result when working with this repository and the scripts it contains:

- The remote `sf` exists on main module and points to `git@github.com:ChainSafe/erigon.git`
- The remote `origin` exists on main module and points to https://github.com/ledgerwatch/erigon.git

### Running Erigon

Erigon can be used as an Execution Layer (EL) for Consensus Layer clients (CL). Default configuration is OK. 

If your CL client is on a different device, add --authrpc.addr 0.0.0.0 (Engine API listens on localhost by default) as well as --authrpc.vhosts <CL host>.

Starting Erigon for goerli chain
```
erigon --datadir=/datadir/erigon/data   --firehose-enabled --chain=goerli --externalcl --private.api.addr=localhost:9090 --http.api=eth,erigon,web3,net,debug,trace,txpool,parity
```

In order to establish a secure connection between the Consensus Layer and the Execution Layer, a JWT secret key is automatically generated.

The JWT secret key will be present in the datadir by default under the name of `jwt.hex`. This piece of info needs to be specified in the Consensus Layer as well in order to establish connection successfully.

Once Erigon is running, you need to point your CL client to <erigon address>:8551, where <erigon address> is either localhost or the IP address of the device running Erigon, and also point to the JWT secret path created by Erigon.

Starting lighthouse for CL
```
lighthouse bn --network goerli --execution-endpoint http://localhost:8551 --execution-jwt /datadir/erigon/data/jwt.hex --disable-deposit-contract-sync --datadir=/datadir/lighthouse --checkpoint-sync-url https://beaconstate-goerli.chainsafe.io --http
```

### Running Erigon with Firehose

Firehose support stdin reader which we can use with Erigon instrumented client. Command to start the instrumented client with Firehose is provided below

```
erigon --datadir=/datadir/erigon/data   --firehose-enabled --chain=goerli --externalcl --private.api.addr=localhost:9090 --http.api=eth,erigon,web3,net,debug,trace,txpool,parity --nat "extip:18.216.37.132"  $@ 2> /datadir/erigon/logs/erigon.log 1> >(./devel/standard-stdin/start.sh -c 2> /datadir/erigon/logs/firehose.log)
```