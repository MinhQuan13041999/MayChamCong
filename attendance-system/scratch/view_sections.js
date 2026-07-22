const fs = require('fs');
const path = require('path');

const htmlContent = fs.readFileSync(path.join(__dirname, '..', 'web', 'index.html'), 'utf8');
const lines = htmlContent.split('\n');

for (let i = 0; i < lines.length; i++) {
  if (lines[i].includes('employees') || lines[i].includes('nhân viên') || lines[i].includes('Nhân viên')) {
    if (lines[i].includes('id=') || lines[i].includes('class=')) {
      console.log(`Line ${i + 1}: ${lines[i].trim()}`);
    }
  }
}
