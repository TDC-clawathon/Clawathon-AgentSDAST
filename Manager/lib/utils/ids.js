const { v7: uuidv7 } = require("uuid");

function newScanId() {
  return uuidv7();
}

module.exports = { newScanId };
