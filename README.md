# HCS CLI

This is a command line tool to interact with [Hedera Consensus Service](https://www.hedera.com/consensus-service).

To get started, you must either have a **`mainnet`** or a **`testnet`** account. You can
register at [portal.hedera.com](https://portal.hedera.com) to get an *account*. You must update
[hedera_env.json](hedera_env.json) with your account info before running the tool.

## Installation

``` shell
$ GO111MODULE=on go get github.com/hashgraph/hcscli
```

## Configure your Hedera environment 

* Open the hedera_env.json file
* Enter your Hedera account (x.z.y format) 
* Enter your private key
* Save the file

## Get your account balance

``` shell
$ hcscli account balance 0.0.100
```
Example output:

``` shell
balance for account 0.0.100 = 10000 ℏ
```

## License Information

Licensed under Apache License,
Version 2.0 – see [LICENSE](LICENSE) in this repo
or [apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)
