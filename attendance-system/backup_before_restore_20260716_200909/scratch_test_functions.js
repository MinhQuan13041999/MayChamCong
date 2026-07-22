const { spawn } = require('child_process');
const http = require('http');
const WebSocket = require('ws');

const chromeProcess = spawn('C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', [
  '--headless',
  '--disable-gpu',
  '--remote-debugging-port=9222',
  'http://localhost:8085'
]);

setTimeout(() => {
  http.get('http://127.0.0.1:9222/json/list', (res) => {
    let data = '';
    res.on('data', chunk => data += chunk);
    res.on('end', () => {
      try {
        const targets = JSON.parse(data);
        const target = targets.find(t => t.url.includes('localhost:8085'));
        if (!target) {
          console.error('Target not found');
          chromeProcess.kill();
          process.exit(1);
        }

        const ws = new WebSocket(target.webSocketDebuggerUrl);

        ws.on('open', () => {
          let id = 1;
          const send = (method, params = {}) => ws.send(JSON.stringify({ id: id++, method, params }));
          
          send('Runtime.enable');
          send('Log.enable');
          send('Page.enable');
          send('DOM.enable');

          ws.on('message', (messageStr) => {
            const msg = JSON.parse(messageStr);
            if (msg.method === 'Runtime.consoleAPICalled') {
              const args = msg.params.args.map(a => a.value || a.description || JSON.stringify(a));
              console.log('[Browser Console]:', ...args);
            }
          });

          setTimeout(() => {
            const testScript = `
              (async () => {
                // Login
                const loginView = document.getElementById('loginView');
                if (loginView && loginView.classList.contains('active')) {
                  document.getElementById('username').value = 'admin';
                  document.getElementById('password').value = 'admin';
                  document.getElementById('loginForm').dispatchEvent(new Event('submit'));
                  await new Promise(r => setTimeout(r, 1000));
                }

                // Switch to Employees
                const navItem = Array.from(document.querySelectorAll('.nav-item')).find(el => el.dataset.view === 'employees');
                if (navItem) navItem.click();
                await new Promise(r => setTimeout(r, 500));

                console.log('--- TEST DIRECT CALL: openPushAllToDeviceModal ---');
                try {
                  openPushAllToDeviceModal();
                  const modal = document.getElementById('pushAllToDeviceModal');
                  console.log('pushAllToDeviceModal active:', modal.classList.contains('active'), 'display:', window.getComputedStyle(modal).display);
                  modal.classList.remove('active');
                } catch (e) {
                  console.error('openPushAllToDeviceModal error:', e.message);
                }

                console.log('--- TEST DIRECT CALL: openBatchEnrollWizard ---');
                try {
                  openBatchEnrollWizard();
                  const modal = document.getElementById('batchEnrollModal');
                  console.log('batchEnrollModal active:', modal.classList.contains('active'), 'display:', window.getComputedStyle(modal).display);
                  modal.classList.remove('active');
                } catch (e) {
                  console.error('openBatchEnrollWizard error:', e.message);
                }

                console.log('--- TEST DIRECT CALL: openPullFromDeviceModal ---');
                try {
                  openPullFromDeviceModal();
                  const modal = document.getElementById('pullFromDeviceModal');
                  console.log('pullFromDeviceModal active:', modal.classList.contains('active'), 'display:', window.getComputedStyle(modal).display);
                  modal.classList.remove('active');
                } catch (e) {
                  console.error('openPullFromDeviceModal error:', e.message);
                }
              })();
            `;

            ws.send(JSON.stringify({
              id: id++,
              method: 'Runtime.evaluate',
              params: { expression: testScript, awaitPromise: true }
            }));

            setTimeout(() => {
              chromeProcess.kill();
              ws.close();
              process.exit(0);
            }, 6000);

          }, 3000);
        });

      } catch (err) {
        console.error(err);
        chromeProcess.kill();
      }
    });
  });
}, 1500);
