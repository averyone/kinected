package http

import (
	"html/template"
)

// linkTemplate is the HTML template for the Plaid Link page.
var linkTemplate = template.Must(template.New("link").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Connect Your Bank Account</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
            padding: 40px;
            max-width: 400px;
            width: 100%;
            text-align: center;
        }
        h1 {
            color: #1a1a2e;
            margin-bottom: 16px;
            font-size: 24px;
        }
        p {
            color: #666;
            margin-bottom: 32px;
            line-height: 1.6;
        }
        .btn {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            padding: 16px 32px;
            font-size: 16px;
            font-weight: 600;
            border-radius: 8px;
            cursor: pointer;
            width: 100%;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 20px rgba(102, 126, 234, 0.4);
        }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
            box-shadow: none;
        }
        .status {
            margin-top: 24px;
            padding: 12px;
            border-radius: 8px;
            display: none;
        }
        .status.success {
            display: block;
            background: #d4edda;
            color: #155724;
        }
        .status.error {
            display: block;
            background: #f8d7da;
            color: #721c24;
        }
        .status.loading {
            display: block;
            background: #e2e3e5;
            color: #383d41;
        }
        .connected-accounts {
            margin-top: 32px;
            text-align: left;
        }
        .connected-accounts h3 {
            color: #1a1a2e;
            margin-bottom: 16px;
            font-size: 16px;
        }
        .account-item {
            padding: 12px;
            background: #f8f9fa;
            border-radius: 8px;
            margin-bottom: 8px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .account-name {
            font-weight: 500;
            color: #1a1a2e;
        }
        .account-mask {
            color: #666;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Connect Your Bank</h1>
        <p>Securely connect your bank account or credit card to get started.</p>
        <button id="linkBtn" class="btn" onclick="startLink()">Connect Account</button>
        <div id="status" class="status"></div>
        <div id="accounts" class="connected-accounts" style="display: none;">
            <h3>Connected Accounts</h3>
            <div id="accountList"></div>
        </div>
    </div>

    <script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
    <script>
        const BASE_URL = '{{.BaseURL}}';
        let linkHandler = null;

        function showStatus(type, message) {
            const status = document.getElementById('status');
            status.className = 'status ' + type;
            status.textContent = message;
        }

        async function startLink() {
            const btn = document.getElementById('linkBtn');
            btn.disabled = true;
            showStatus('loading', 'Initializing...');

            try {
                // Get link token
                const tokenResp = await fetch(BASE_URL + '/link/token', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include'
                });

                if (!tokenResp.ok) {
                    throw new Error('Failed to create link token');
                }

                const { link_token } = await tokenResp.json();

                // Initialize Plaid Link
                linkHandler = Plaid.create({
                    token: link_token,
                    onSuccess: async (publicToken, metadata) => {
                        showStatus('loading', 'Connecting your account...');

                        try {
                            const exchangeResp = await fetch(BASE_URL + '/link/exchange', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                credentials: 'include',
                                body: JSON.stringify({
                                    public_token: publicToken,
                                    institution_id: metadata.institution.institution_id,
                                    institution_name: metadata.institution.name
                                })
                            });

                            if (!exchangeResp.ok) {
                                throw new Error('Failed to exchange token');
                            }

                            const item = await exchangeResp.json();
                            showStatus('success', 'Account connected successfully!');
                            loadAccounts();
                        } catch (err) {
                            showStatus('error', 'Failed to connect account: ' + err.message);
                        }
                    },
                    onExit: (err, metadata) => {
                        btn.disabled = false;
                        if (err) {
                            showStatus('error', 'Link closed: ' + (err.display_message || err.error_message));
                        } else {
                            document.getElementById('status').style.display = 'none';
                        }
                    },
                    onEvent: (eventName, metadata) => {
                        console.log('Plaid event:', eventName, metadata);
                    }
                });

                linkHandler.open();
            } catch (err) {
                btn.disabled = false;
                showStatus('error', 'Error: ' + err.message);
            }
        }

        async function loadAccounts() {
            try {
                const resp = await fetch(BASE_URL + '/accounts', {
                    credentials: 'include'
                });

                if (!resp.ok) return;

                const { accounts } = await resp.json();

                if (accounts && accounts.length > 0) {
                    const container = document.getElementById('accounts');
                    const list = document.getElementById('accountList');

                    list.innerHTML = accounts.map(acc => ` + "`" + `
                        <div class="account-item">
                            <span class="account-name">${acc.name}</span>
                            <span class="account-mask">****${acc.mask || '----'}</span>
                        </div>
                    ` + "`" + `).join('');

                    container.style.display = 'block';
                    document.getElementById('linkBtn').textContent = 'Connect Another Account';
                    document.getElementById('linkBtn').disabled = false;
                }
            } catch (err) {
                console.error('Failed to load accounts:', err);
            }
        }

        // Load existing accounts on page load
        loadAccounts();
    </script>
</body>
</html>`))

// callbackTemplate is the HTML template for the OAuth callback page.
var callbackTemplate = template.Must(template.New("callback").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Connecting...</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .container {
            text-align: center;
            padding: 40px;
        }
        .spinner {
            width: 40px;
            height: 40px;
            border: 3px solid #e0e0e0;
            border-top-color: #667eea;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 20px;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="spinner"></div>
        <p>Completing connection...</p>
    </div>
    <script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
    <script>
        // This page handles OAuth redirects
        // Plaid Link will automatically resume when this page loads
        const linkHandler = Plaid.create({
            token: null, // Will be restored from localStorage by Plaid
            receivedRedirectUri: window.location.href
        });
    </script>
</body>
</html>`))
