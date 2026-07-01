/**
 * GlobalRemit - Checkout Orchestration for CyberSource Flex Microform & 3DS 2.x
 * Version: 2.0.0 — Supports Saved Cards (Card-on-File)
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

    // Saved Cards Elements
    const savedCardsSection = document.getElementById('saved-cards-section');
    const savedCardsList = document.getElementById('saved-cards-list');
    const newCardContainer = document.getElementById('new-card-container');
    const useNewCardRadio = document.getElementById('use-new-card');

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
    let savedCards = [];
    let fingerprintID = null;

    // ─── Initial Load ────────────────────────────────────────────────────────
    fetchRate();
    initProfiling();

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
            accountNumber.required = false;
        } else {
            accountGroup.style.display = 'flex';
            accountLabel.textContent = type === 'WITHIN_BOA' ? 'BoA Account Number' : 'Bank Account Number';
            accountNumber.required = true;
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
        approxReceiveDisplay.textContent = `${total.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ETB`;
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

    async function initProfiling() {
        try {
            const resp = await fetch('/api/collection/config');
            const cfg = await resp.json();
            const merchantID = cfg.merchant_id;
            
            // Generate a unique session ID
            const sessionID = merchantID + Date.now() + Math.random().toString(36).substring(2, 9);
            fingerprintID = sessionID;

            console.log("INFO: Initiating CyberSource Device Profiling. Session ID:", sessionID);
            console.log("DFP===>", sessionID);

            const orgID = "1s4h7ay3"; // Sandbox Org ID
            
            // Inject profiling scripts/tags
            // Source: https://developer.cybersource.com/library/documentation/dev_guides/Decision_Manager_Using_REST_API/html/topics/device_fingerprinting.htm
            const pDiv = document.createElement('div');
            pDiv.style.background = `url(https://tmtest.cybersource.com/fp/clear.png?org_id=${orgID}&session_id=${sessionID}&m=1)`;
            pDiv.style.width = "1px";
            pDiv.style.height = "1px";
            
            const img = document.createElement('img');
            img.src = `https://tmtest.cybersource.com/fp/clear.png?org_id=${orgID}&session_id=${sessionID}&m=2`;
            img.alt = "";
            pDiv.appendChild(img);

            const script = document.createElement('script');
            script.src = `https://tmtest.cybersource.com/fp/check.js?org_id=${orgID}&session_id=${sessionID}`;
            pDiv.appendChild(script);

            document.body.appendChild(pDiv);
            
        } catch (err) {
            console.error("ERROR: Failed to initiate profiling:", err);
        }
    }

    // ─── Fetch Saved Cards ──────────────────────────────────────────────────
    async function fetchSavedCards(email) {
        if (!email) return [];
        try {
            const response = await fetch(`/api/collection/saved-cards?email=${encodeURIComponent(email)}`);
            if (!response.ok) return [];
            const data = await response.json();
            return Array.isArray(data) ? data : [];
        } catch (err) {
            console.error('Failed to fetch saved cards', err);
            return [];
        }
    }

    function getCardBrandIcon(brand) {
        switch (brand) {
            case '001': return '💳'; // Visa
            case '002': return '💳'; // Mastercard
            case '003': return '💳'; // Amex
            default: return '💳';
        }
    }

    function getCardBrandName(brand) {
        switch (brand) {
            case '001': return 'Visa';
            case '002': return 'Mastercard';
            case '003': return 'Amex';
            default: return 'Card';
        }
    }

    function renderSavedCards(cards) {
        savedCardsList.innerHTML = '';
        selectedSavedCard = null;

        if (!cards || cards.length === 0) {
            savedCardsSection.classList.add('hidden');
            newCardContainer.classList.remove('hidden');
            return;
        }

        savedCardsSection.classList.remove('hidden');

        cards.forEach((card, index) => {
            const suffix = card.card_suffix || '****';
            const brandName = getCardBrandName(card.card_brand);
            const brandIcon = getCardBrandIcon(card.card_brand);

            const label = document.createElement('label');
            label.className = 'saved-card-option';
            label.innerHTML = `
                <input type="radio" name="payment-method" value="saved-${index}" data-token-id="${card.token_id}" data-exp-month="${card.expiration_month || ''}" data-exp-year="${card.expiration_year || ''}">
                <div class="saved-card-info">
                    <span class="card-icon">${brandIcon}</span>
                    <span class="card-detail">
                        ${brandName} •••• ${suffix}
                        <span class="card-detail-sub">Token: ${card.token_id.substring(0, 8)}...</span>
                    </span>
                </div>
            `;
            savedCardsList.appendChild(label);
        });

        // Default to first saved card
        const firstRadio = savedCardsList.querySelector('input[type="radio"]');
        if (firstRadio) {
            firstRadio.checked = true;
            selectedSavedCard = {
                tokenId: firstRadio.dataset.tokenId,
                expirationMonth: firstRadio.dataset.expMonth,
                expirationYear: firstRadio.dataset.expYear
            };
            newCardContainer.classList.add('hidden');
            useNewCardRadio.checked = false;
        }
    }

    // ─── Payment Method Radio Toggle ───────────────────────────────────────
    document.addEventListener('change', (e) => {
        if (e.target.name !== 'payment-method') return;

        if (e.target.value === 'new') {
            selectedSavedCard = null;
            newCardContainer.classList.remove('hidden');
            // Re-initialize Microform if needed
            if (captureContext && !microform) {
                initializeMicroform(captureContext);
            }
        } else {
            selectedSavedCard = {
                tokenId: e.target.dataset.tokenId,
                expirationMonth: e.target.dataset.expMonth,
                expirationYear: e.target.dataset.expYear
            };
            newCardContainer.classList.add('hidden');
        }
    });

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
            paymentError.classList.add('hidden');

            // Step 1b: Fetch saved cards for this sender
            const senderEmail = document.getElementById('sender_email').value;
            savedCards = await fetchSavedCards(senderEmail);
            renderSavedCards(savedCards);

            // Show the modal
            paymentModal.classList.remove('hidden');

            // Step 2: Only initialize Microform if no saved cards or user picks new card
            if (savedCards.length === 0 || !selectedSavedCard) {
                await loadFlexSDK();
                initializeMicroform(captureContext);
            }

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

        const senderData = {
            first_name: document.getElementById('sender_name').value.split(' ')[0],
            last_name: document.getElementById('sender_name').value.split(' ').slice(1).join(' ') || 'Sender',
            email: document.getElementById('sender_email').value,
            address: document.getElementById('sender_address').value,
            city: document.getElementById('sender_city').value,
            administrative_area: document.getElementById('sender_state').value,
            country: document.getElementById('sender_country').value,
            postal_code: document.getElementById('sender_postal').value
        };

        const recipientData = {
            first_name: document.getElementById('receiver_name').value.split(' ')[0] || 'Recipient',
            last_name: document.getElementById('receiver_name').value.split(' ').slice(1).join(' ') || 'Recipient',
            address: document.getElementById('receiver_address').value,
            city: document.getElementById('receiver_city').value,
            country: document.getElementById('receiver_country').value
        };

        // Validate that we have a remittance ID from the initiation step
        // Validate that we have a remittance ID from the initiation step
        const id = currentRemittance.id
        if (!id) {
            console.error('ID MISSING', currentRemittance);
            alert('ID MISSING: ' + JSON.stringify(currentRemittance));
            showPaymentError('Technical error: No Transaction Reference found. Please refresh and try again.');
            setPaymentLoading(false);
            return;
        }

        // ── SAVED CARD FLOW ──
        if (selectedSavedCard) {
            try {
                // PA Setup with permanent token
                const paSetupBody = {
                    id: id,
                    permanent_token_id: selectedSavedCard.tokenId
                };

                const paSetupResp = await fetch('/api/collection/pa-setup', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(paSetupBody)
                });

                const paData = await paSetupResp.json();

                // DDC
                await performDDC(paData.device_data_collection_url, paData.access_token);

                // Browser info for 3DS 2.x
                const browserInfo = {
                    userAgentBrowserValue: navigator.userAgent,
                    httpBrowserColorDepth: String(screen.colorDepth),
                    httpBrowserScreenWidth: String(screen.width),
                    httpBrowserScreenHeight: String(screen.height),
                    httpBrowserLanguage: navigator.language || navigator.userLanguage,
                    httpBrowserTimeDifference: String(new Date().getTimezoneOffset()),
                    httpBrowserJavaEnabled: navigator.javaEnabled ? navigator.javaEnabled() : false,
                    httpBrowserJavaScriptEnabled: true,
                    httpAcceptBrowserValue: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"
                };
                console.log("INFO: Browser Info for 3DS 2.x:", browserInfo);

                // Authorization with permanent token
                await processAuthorization({
                    id: id,
                    permanent_token_id: selectedSavedCard.tokenId,
                    pa_reference_id: paData.reference_id,
                    browser_info: browserInfo,
                    fingerprint_id: fingerprintID
                });

            } catch (err) {
                showPaymentError(err.message);
                setPaymentLoading(false);
            }
            return;
        }

        // ── NEW CARD FLOW ──
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
                    jti = token; // Last resort
                }
            }


            try {
                // Step 4: PA Setup
                const paSetupBody = {
                    id: id,
                    transient_token_jti: jti,
                    transient_token_jwt: token
                };


                const paSetupResp = await fetch('/api/collection/pa-setup', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(paSetupBody)
                });

                const paData = await paSetupResp.json();

                // Step 5: Device Data Collection (DDC)
                await performDDC(paData.device_data_collection_url, paData.access_token);

                // Collect browser information for 3DS 2.x Browser Flow
                const browserInfo = {
                    userAgentBrowserValue: navigator.userAgent,
                    httpBrowserColorDepth: String(screen.colorDepth),
                    httpBrowserScreenWidth: String(screen.width),
                    httpBrowserScreenHeight: String(screen.height),
                    httpBrowserLanguage: navigator.language || navigator.userLanguage,
                    httpBrowserTimeDifference: String(new Date().getTimezoneOffset()),
                    httpBrowserJavaEnabled: navigator.javaEnabled ? navigator.javaEnabled() : false,
                    httpBrowserJavaScriptEnabled: true,
                    httpAcceptBrowserValue: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"
                };

                // Step 6: Authorization Request
                await processAuthorization({
                    id: id,
                    transient_token_jti: jti,
                    transient_token_jwt: token,
                    pa_reference_id: paData.reference_id,
                    browser_info: browserInfo,
                    fingerprint_id: fingerprintID
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
        console.log("AUTH_REQUEST_SENT===>", authReq);
        if (authReq.fingerprint_id) {
            console.log("DFP===>", authReq.fingerprint_id);
        }
        const response = await fetch('/api/collection/authorize', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(authReq)
        });

        const authResp = await response.json();

        if (authResp.status === 'AUTHORIZED' || authResp.status === 'AUTHORIZED_PENDING_REVIEW') {
            const redirectUrl = authResp.status === 'AUTHORIZED_PENDING_REVIEW' ? '/checkout/review' : '/checkout/success';
            window.location.href = `${redirectUrl}?ref=${authReq.id}`;
        } else if (authResp.status === 'PENDING_AUTHENTICATION') {
            // STEP 7: Challenge Required
            await handleChallenge(authResp.step_up_url, authResp.access_token, authResp.authentication_transaction_id, authReq);
        } else {
            throw new Error(authResp.message || 'Payment declined');
        }
    }

    function handleChallenge(url, jwt, txnId, authReq) {
        paymentModal.classList.add('hidden');
        challengeModal.classList.remove('hidden');

        challengeContainer.innerHTML = `
            <form id="challenge-form" method="POST" action="${url}" target="challenge-iframe">
                <input type="hidden" name="JWT" value="${jwt}">
            </form>
            <iframe name="challenge-iframe" width="100%" height="500" border="0" style="border-radius: 12px; border: 1px solid var(--color-border);"></iframe>
        `;

        document.getElementById('challenge-form').submit();

        // Note: No further logic needed here as the /collection/return handler 
        // will handle the post-3DS redirect to success/error pages.
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
