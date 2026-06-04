/**
 * GlobalRemit - Checkout Orchestration for CyberSource Flex Microform & 3DS 2.x
 * Version: 1.2.1
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
                opt.value = bank.id;
                opt.textContent = bank.institutionName;
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

        let year = expYear.value;
        if (year && year.length === 2) {
            year = '20' + year;
        }

        let month = expMonth.value;
        if (month && month.length === 1) {
            month = '0' + month;
        }

        const options = {
            expirationMonth: month,
            expirationYear: year
        };

        microform.createToken(options, async (err, tokenResponse) => {
            if (err) {
                showPaymentError(err.message || 'Check card details');
                setPaymentLoading(false);
                return;
            }

            // Normalize token response
            const token = tokenResponse.token || (typeof tokenResponse === 'string' ? tokenResponse : '');
            let jti = tokenResponse.jti;

            // Fallback: If JTI is missing, parse it from the JWT payload
            if (!jti && token.includes('.')) {
                try {
                    const base64Url = token.split('.')[1];
                    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
                    const payload = JSON.parse(window.atob(base64));
                    jti = payload.jti;
                } catch (e) {
                    console.error('DEBUG: Failed to parse JTI from JWT:', e);
                    jti = token; // Last resort
                }
            }

            
            try {
                // Step 4: PA Setup
                const paSetupBody = {
                    remittance_id: currentRemittance.remittance_id,
                    transient_token_jti: jti,
                    transient_token_jwt: token,
                    expiration_month: month,
                    expiration_year: year
                };
                

                const paSetupResp = await fetch('/api/collection/pa-setup', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(paSetupBody)
                });
                
                const paData = await paSetupResp.json();

                // Step 5: Device Data Collection (DDC)
                await performDDC(paData.device_data_collection_url, paData.access_token);

                // Step 6: Authorization Request
                await processAuthorization({
                    remittance_id: currentRemittance.remittance_id,
                    transient_token_jti: jti,
                    transient_token_jwt: token,
                    expiration_month: month,
                    expiration_year: year,
                    pa_reference_id: paData.reference_id,
                    amount: parseFloat(document.getElementById('send_amount').value).toFixed(2),
                    currency: currentRemittance.send_currency,
                    sender: {
                        first_name: document.getElementById('sender_name').value.split(' ')[0],
                        last_name: document.getElementById('sender_name').value.split(' ').slice(1).join(' ') || 'Sender',
                        email: document.getElementById('sender_email').value,
                        address: document.getElementById('sender_address').value,
                        city: document.getElementById('sender_city').value,
                        administrative_area: document.getElementById('sender_state').value,
                        country: document.getElementById('sender_country').value,
                        postal_code: document.getElementById('sender_postal').value
                    },
                    recipient: {
                        first_name: document.getElementById('receiver_name').value.split(' ')[0] || 'Recipient',
                        last_name: document.getElementById('receiver_name').value.split(' ').slice(1).join(' ') || 'Recipient',
                        address: document.getElementById('receiver_address').value,
                        city: document.getElementById('receiver_city').value,
                        country: document.getElementById('receiver_country').value
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
            
            // CyberSource DDC completion listener
            const handleDDCMessage = (event) => {
                try {
                    const data = typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
                    if (data && data.MessageType === 'profile.completed') {
                        console.log('DDC Profiling completed');
                        window.removeEventListener('message', handleDDCMessage);
                        resolve();
                    }
                } catch (e) {
                    // Ignore non-JSON messages
                }
            };
            
            window.addEventListener('message', handleDDCMessage);
            
            const form = document.getElementById('ddc-form');
            form.submit();

            // Fallback timeout after 5 seconds just in case
            setTimeout(() => {
                window.removeEventListener('message', handleDDCMessage);
                resolve();
            }, 5000);
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

        if (authResp.status === 'AUTHORIZED' || authResp.status === 'AUTHORIZED_PENDING_REVIEW') {
            const redirectUrl = authResp.status === 'AUTHORIZED_PENDING_REVIEW' ? '/checkout/review' : '/checkout/success';
            window.location.href = `${redirectUrl}?ref=${currentRemittance.remittance_id}`;
        } else if (authResp.status === 'PENDING_AUTHENTICATION') {
            // STEP 7: Challenge Required
            await handleChallenge(authResp.step_up_url, authResp.access_token, authResp.authentication_transaction_id);
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
