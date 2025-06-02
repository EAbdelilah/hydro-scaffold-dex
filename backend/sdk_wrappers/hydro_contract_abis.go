package sdk_wrappers

// HydroContractABIJsonString contains the JSON string representation of the Hydro contract ABI.
// TODO: Replace this placeholder with the actual full Hydro contract ABI JSON string.
const HydroContractABIJsonString = `
[
  {
    "constant": true,
    "inputs": [
      { "name": "user", "type": "address" },
      { "name": "marketID", "type": "uint16" }
    ],
    "name": "getAccountDetails",
    "outputs": [
      { "name": "liquidatable", "type": "bool" },
      { "name": "status", "type": "uint8" },
      { "name": "debtsTotalUSDValue", "type": "uint256" },
      { "name": "assetsTotalUSDValue", "type": "uint256" }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      { "name": "marketID", "type": "uint16" },
      { "name": "asset", "type": "address" },
      { "name": "user", "type": "address" }
    ],
    "name": "marketBalanceOf",
    "outputs": [
      { "name": "balance", "type": "uint256" }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      { "name": "marketID", "type": "uint16" },
      { "name": "asset", "type": "address" },
      { "name": "user", "type": "address" }
    ],
    "name": "getMarketTransferableAmount",
    "outputs": [
      { "name": "amount", "type": "uint256" }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "components": [
          { "name": "actionType", "type": "uint8" },
          { "name": "encodedParams", "type": "bytes" }
        ],
        "name": "actions",
        "type": "tuple[]"
      }
    ],
    "name": "batch",
    "outputs": [],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  }
]
`

// MarginContractABIJsonString contains a key-feature subset of the full Margin contract ABI.
// Developer MUST ensure the true, complete ABI is manually placed here later if this subset is insufficient.
const MarginContractABIJsonString = `[
  {"constant":true,"inputs":[{"name":"user","type":"address"},{"name":"marketID","type":"uint16"}],"name":"getAccountDetails","outputs":[{"components":[{"name":"liquidatable","type":"bool"},{"name":"status","type":"uint8"},{"name":"debtsTotalUSDValue","type":"uint256"},{"name":"balancesTotalUSDValue","type":"uint256"}],"name":"details","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"},
  {"constant":true,"inputs":[{"name":"asset","type":"address"},{"name":"user","type":"address"},{"name":"marketID","type":"uint16"}],"name":"getAmountBorrowed","outputs":[{"name":"amount","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},
  {"constant":true,"inputs":[{"name":"assetAddress","type":"address"}],"name":"getAsset","outputs":[{"components":[{"name":"lendingPoolToken","type":"address"},{"name":"priceOracle","type":"address"},{"name":"interestModel","type":"address"}],"name":"asset","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"},
  {"constant":false,"inputs":[{"components":[{"name":"actionType","type":"uint8"},{"name":"encodedParams","type":"bytes"}],"name":"actions","type":"tuple[]"}],"name":"batch","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},
  {"constant":true,"inputs":[{"name":"marketID","type":"uint16"}],"name":"getMarket","outputs":[{"components":[{"name":"baseAsset","type":"address"},{"name":"quoteAsset","type":"address"},{"name":"liquidateRate","type":"uint256"},{"name":"withdrawRate","type":"uint256"},{"name":"auctionRatioStart","type":"uint256"},{"name":"auctionRatioPerBlock","type":"uint256"},{"name":"borrowEnable","type":"bool"}],"name":"market","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"},
  {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"IncreaseCollateral","type":"event"},
  {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"DecreaseCollateral","type":"event"},
  {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Borrow","type":"event"},
  {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Repay","type":"event"}
]`;


const GenericPriceOracleABIJsonString = `[
    {
        "constant": true,
        "inputs": [{"name": "asset", "type": "address"}],
        "name": "getPrice",
        "outputs": [{"name": "price", "type": "uint256"}],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    }
]`
