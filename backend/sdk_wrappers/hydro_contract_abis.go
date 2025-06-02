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

// MarginContractABIJsonString contains the JSON string representation of the new Margin contract ABI.
// TODO: Replace this placeholder with the actual full Margin contract ABI JSON string.
const MarginContractABIJsonString = `[{"constant":true,"inputs":[],"name":"getAccountDetails","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}, {"name":"batch","inputs":[{"components":[{"name":"actionType","type":"uint8"},{"name":"encodedParams","type":"bytes"}],"name":"actions","type":"tuple[]"}],"outputs":[],"payable":true,"stateMutability":"payable","type":"function"} ]`
