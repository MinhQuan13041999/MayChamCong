const fs = require('fs');

const appJs = fs.readFileSync('web/app.js', 'utf8');
const indexHtml = fs.readFileSync('web/index.html', 'utf8');

// Find all document.getElementById('...') in app.js
const regex = /document\.getElementById\(['"]([^'"]+)['"]\)/g;
let match;
const idsInJs = new Set();
while ((match = regex.exec(appJs)) !== null) {
  idsInJs.add(match[1]);
}

console.log(`Found ${idsInJs.size} unique IDs requested in app.js.`);

let missingCount = 0;
for (const id of idsInJs) {
  // Check if id is present in index.html as id="id" or id='id'
  const escapedId = id.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&');
  const idRegex = new RegExp(`id=["']${escapedId}["']`, 'i');
  if (!idRegex.test(indexHtml)) {
    console.log(`❌ Missing ID in HTML: ${id}`);
    missingCount++;
  }
}

if (missingCount === 0) {
  console.log('✅ All IDs requested in app.js exist in index.html!');
} else {
  console.log(`❌ Total missing IDs: ${missingCount}`);
}
