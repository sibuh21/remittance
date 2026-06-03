/**
 * GlobalRemit - Checkout Orchestration for CyberSource Flex Microform & 3DS 2.x
 */

document.addEventListener('DOMContentLoaded', () => {
    // --- Elements ---
    const remitForm = document.getElementById('remit-form');
    const submitBtn = document.getElementById('submit-btn');
    const sendAmount = document.getElementById('send_amount');
    const sendCurrency = document.getElementById('send_currency');
    const payoutType = document.getElementById('payout_type');
    const bankSelectGroup = document.getElementById('bank-select-group');
    const bankId = document.getElementById('bank_id');
    const accountGroup = document.getElementById('account-group');
    const accountLabel = document.getElementById('account-label');
    const accountNumber = document.getElementById('account_number');
    const phoneGroup = document.getElementById('phone-group');
    const receiverPhone = document.getElementById('receiver_phone');
    const currentRateDisplay = document.getElementById('current-rate');
    const approxReceiveDisplay = document.getElementById('approx-receive');

    // Payment Modal Elements
    const paymentModal = document.getElementById('payment-modal');
    const closePaymentModal = document.getElementById('close-payment-modal');
    const modalTotal = document.getElementById('modal-total');
    const authorizeBtn = document.getElementById('authorize-btn');
    const paymentError = document.getElementById('payment-error');
    const expMonth = document.getElementById('exp-month');
    const expYear = document.getElementById('exp-year');

    // 3DS Challenge Elements
    const challengeModal = document.getElementById('challenge-modal');
    const challengeContainer = document.getElementById('challenge-container');
    const cancelChallenge = document.getElementById('cancel-challenge');

    // --- State ---
    let currentRate = 0;
    let flexInstance = null;
    let microform = null;
    let currentRemittance = null;
    let captureContext = null;

    // ─── Initial Load ────────────────────────────────────────────────────────
    fetchRate();

    // ─── SDK Loading ─────────────────────────────────────────────────────────
    const FLEX_SDK_URL = 'https://flex.cybersource.com/cybersource/assets/microform/0.11/flex-microform.min.js';
    const loadFlexSDK = () => {
        return new Promise((resolve, reject) => {
            if (window.Flex) return resolve();
            const script = document.createElement('script');
            script.src = FLEX_SDK_URL;
            script.onload = resolve;
            script.onerror = reject;
            document.head.appendChild(script);
        });
    };

    // ─── Payout Type Toggles ───────────────────────────────────────────────
    payoutType.addEventListener('change', () => {
        const type = payoutType.value;
        if (type === 'OTHER_BANK') {
            bankSelectGroup.style.display = 'flex';
            loadBanks();
        } else {
            bankSelectGroup.style.display = 'none';
        }

        if (type === 'TELEBIRR' || type === 'MPESA') {
            accountGroup.style.display = 'none';
            phoneGroup.style.display = 'flex';
            accountNumber.required = false;
            receiverPhone.required = true;
        } else {
            accountGroup.style.display = 'flex';
            phoneGroup.style.display = 'none';
            accountLabel.textContent = type === 'WITHIN_BOA' ? 'BoA Account Number' : 'Bank Account Number';
            accountNumber.required = true;
            receiverPhone.required = false;
        }
    });

    // ─── Exchange Rate Logic ────────────────────────────────────────────────
    async function fetchRate() {
        const currency = sendCurrency.value;
        try {
            const response = await fetch(`/api/payout/rate/${currency}`);
            const data = await response.json();
            currentRate = data.rate || 0;
            currentRateDisplay.textContent = `1 ${currency} = ${currentRate.toFixed(4)} ETB`;
            updateReceiveAmount();
        } catch (err) {
            currentRateDisplay.textContent = 'Rate unavailable';
        }
    }

    function updateReceiveAmount() {
        const amount = parseFloat(sendAmount.value) || 0;
        const total = amount * currentRate;
        approxReceiveDisplay.textContent = `${total.toLocaleString(undefined, {minimumFractionDigits: 2, maximumFractionDigits: 2})} ETB`;
    }

    sendAmount.addEventListener('input', updateReceiveAmount);
    sendCurrency.addEventListener('change', fetchRate);

    // ─── Load Banks ─────────────────────────────────────────────────────────
    async function loadBanks() {
        if (bankId.options.length > 1) return;
        try {
            const response = await fetch('/api/payout/banks');
            const data = await response.json();
            data.banks.forEach(bank => {
                const opt = document.createElement('option');
                opt.value = bank.bankId;
                opt.textContent = bank.bankName;
                bankId.appendChild(opt);
            });
        } catch (err) {
            console.error('Failed to load banks', err);
        }
    }

    // ─── Stage 1: Initiation ───────────────────────────────────────────────
    remitForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        setLoading(true);

        const formData = new FormData(remitForm);
        const requestBody = Object.fromEntries(formData.entries());
        requestBody.target_currency = "ETB";

        try {
            // Step 1: Backend Initiation
            const response = await fetch('/api/remittance', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            });

            if (!response.ok) {
                const err = await response.json();
                throw new Error(err.message || 'Initiation failed');
            }

            currentRemittance = await response.json();
            captureContext = currentRemittance.capture_context;

            // Prepare Modal
            modalTotal.textContent = `${currentRemittance.send_amount} ${currentRemittance.send_currency}`;
            paymentModal.classList.remove('hidden');

            // Step 2: Initialize Flex Microform
            await loadFlexSDK();
            initializeMicroform(captureContext);

        } catch (err) {
            alert(err.message);
        } finally {
            setLoading(false);
        }
    });

    // ─── Stage 2: Initialize Flex Microform ───────────────────────────────
    function initializeMicroform(jwt) {
        // Clear previous iframes if any
        document.getElementById('card-number-container').innerHTML = '';
        document.getElementById('security-code-container').innerHTML = '';

        flexInstance = new Flex(jwt);
        microform = flexInstance.microform();

        const style = {
            'input': {
                'font-family': 'Inter, system-ui, sans-serif',
                'font-size': '16px',
                'color': '#f1f5f9'
            },
            ':focus': { 'color': '#6366f1' },
            ':disabled': { 'cursor': 'not-allowed' },
            'valid': { 'color': '#22c55e' },
            'invalid': { 'color': '#ef4444' }
        };

        const cardNumber = microform.createField('number', { placeholder: '•••• •••• •••• ••••', style });
        const securityCode = microform.createField('securityCode', { placeholder: '•••', style });

        cardNumber.load('#card-number-container');
        securityCode.load('#security-code-container');
    }

    // ─── Stage 3: Tokenization & Authentication ────────────────────────────
    authorizeBtn.addEventListener('click', async () => {
        setPaymentLoading(true);
        paymentError.classList.add('hidden');

        const options = {
            expirationMonth: expMonth.value,
            expirationYear: expYear.value
        };

        microform.createToken(options, async (err, tokenResponse) => {
            if (err) {
                showPaymentError(err.message || 'Check card details');
                setPaymentLoading(false);
                return;
            }

            console.log('Transient Token created:', tokenResponse.jti);
            
            try {
                // Step 4: PA Setup
                const paSetupResp = await fetch('/api/collection/pa-setup', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        remittance_id: currentRemittance.remittance_id,
                        transient_token_jti: tokenResponse.jti
                    })
                });
                
                const paData = await paSetupResp.json();

                // Step 5: Device Data Collection (DDC)
                await performDDC(paData.deviceDataCollectionUrl, paData.accessToken);

                // Step 6: Authorization Request
                await processAuthorization({
                    remittance_id: currentRemittance.remittance_id,
                    transient_token_jwt: tokenResponse.token,
                    pa_reference_id: paData.referenceId,
                    amount: currentRemittance.send_amount,
                    currency: currentRemittance.send_currency,
                    sender: {
                        first_name: document.getElementById('sender_name').value.split(' ')[0],
                        last_name: document.getElementById('sender_name').value.split(' ').slice(1).join(' ') || 'Sender',
                        email: document.getElementById('sender_email').value,
                        address: 'Global Remit Customer', // Simplified for demo
                        city: 'Internet',
                        country: document.getElementById('sender_country').value || 'USA',
                        country_iso3: 'USA', // Map as needed
                        postal_code: '10001'
                    },
                    recipient: {
                        first_name: document.getElementById('receiver_name').value.split(' ')[0],
                        last_name: document.getElementById('receiver_name').value.split(' ').slice(1).join(' ') || 'Recipient',
                        country_iso3: 'ETH'
                    }
                });

            } catch (err) {
                showPaymentError(err.message);
                setPaymentLoading(false);
            }
        });
    });

    // ─── Step 5: DDC (Device Data Collection) ───────────────────────────────
    function performDDC(url, jwt) {
        return new Promise((resolve) => {
            const container = document.getElementById('ddc-container');
            container.innerHTML = `
                <form id="ddc-form" method="POST" action="${url}" target="ddc-iframe">
                    <input type="hidden" name="JWT" value="${jwt}">
                </form>
                <iframe name="ddc-iframe" style="display:none;"></iframe>
            `;
            
            const form = document.getElementById('ddc-form');
            form.submit();

            // Wait for DDC to finish (usually 2 seconds is enough for background profiling)
            setTimeout(resolve, 2000);
        });
    }

    // ─── Step 6 & 7: Authorization & Challenge ──────────────────────────────
    async function processAuthorization(authReq) {
        const response = await fetch('/api/collection/authorize', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(authReq)
        });

        const authResp = await response.json();

        if (authResp.status === 'AUTHORIZED') {
            window.location.href = `/checkout/success?ref=${currentRemittance.remittance_id}`;
        } else if (authResp.status === 'PENDING_AUTHENTICATION') {
            // STEP 7: Challenge Required
            await handleChallenge(authResp.stepUpUrl, authResp.accessToken, authResp.authenticationTransactionId);
        } else {
            throw new Error(authResp.message || 'Payment declined');
        }
    }

    function handleChallenge(url, jwt, txnId) {
        return new Promise((resolve, reject) => {
            paymentModal.classList.add('hidden');
            challengeModal.classList.remove('hidden');

            challengeContainer.innerHTML = `
                <form id="challenge-form" method="POST" action="${url}" target="challenge-iframe">
                    <input type="hidden" name="JWT" value="${jwt}">
                </form>
                <iframe name="challenge-iframe" width="100%" height="400" border="0"></iframe>
            `;

            document.getElementById('challenge-form').submit();

            // Listen for completion (Simplification: CyberSource redirects the iframe back to our ReturnURL)
            // For a real SPA, we'd listen for a postMessage or poll
            // Here, we'll assume the browser will handle the redirect within the iframe.
            // But we need to call ValidateAndAuthorize after the user finishes.
            
            // For this demo, let's provide a "I have finished" button OR use a timeout/message listener.
            window.onchallengecomplete = async () => {
                try {
                    const validateResp = await fetch('/api/collection/validate', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            remittance_id: currentRemittance.remittance_id,
                            authentication_transaction_id: txnId
                        })
                    });
                    const finalData = await validateResp.json();
                    if (finalData.status === 'AUTHORIZED') {
                        window.location.href = `/checkout/success?ref=${currentRemittance.remittance_id}`;
                    } else {
                        throw new Error('3DS Validation failed');
                    }
                } catch (err) {
                    showPaymentError(err.message);
                    challengeModal.classList.add('hidden');
                    paymentModal.classList.remove('hidden');
                    setPaymentLoading(false);
                }
            };
        });
    }

    // ─── UI Helpers ────────────────────────────────────────────────────────
    function setLoading(isLoading) {
        submitBtn.disabled = isLoading;
        submitBtn.textContent = isLoading ? 'Initiating...' : 'Continue to Payment';
    }

    function setPaymentLoading(isLoading) {
        authorizeBtn.disabled = isLoading;
        authorizeBtn.textContent = isLoading ? 'Authorizing...' : 'Authorize Payment';
    }

    function showPaymentError(msg) {
        paymentError.textContent = msg;
        paymentError.classList.remove('hidden');
    }

    closePaymentModal.addEventListener('click', () => {
        paymentModal.classList.add('hidden');
    });

    cancelChallenge.addEventListener('click', () => {
        challengeModal.classList.add('hidden');
        paymentModal.classList.remove('hidden');
        setPaymentLoading(false);
    });
});
