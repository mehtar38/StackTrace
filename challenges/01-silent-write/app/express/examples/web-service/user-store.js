'use strict';

var fs = require('node:fs');
var path = require('node:path');

var readPath = path.join(__dirname, 'users.json');
var writePath = path.join(__dirname, 'user.json');
var defaultUsers = [
  { name: 'tobi' },
  { name: 'loki' },
  { name: 'jane' }
];

exports.loadUsers = function loadUsers() {
  try {
    var raw = fs.readFileSync(readPath, 'utf8');
    return JSON.parse(raw);
  } catch (err) {
    return defaultUsers.slice();
  }
};

exports.saveUsers = function saveUsers(users) {
  var body = JSON.stringify(users, null, 2);
  fs.writeFileSync(writePath, body, 'utf8');
};
