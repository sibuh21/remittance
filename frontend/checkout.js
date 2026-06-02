document.addEventListener('DOMContentLoaded', () => {
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

    let currentRate = 0;

    // ─── Payout Type Toggles ───────────────────────────────────────────────
    payoutType.addEventListener('change', () => {
        const type = payoutType.value;
        
        // Hide/Show banks
        if (type === 'OTHER_BANK') {
            bankSelectGroup.style.display = 'flex';
            loadBanks();
        } else {
            bankSelectGroup.style.display = 'none';
        }

        // Adjust Account/Phone labels
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

    // ─── Form Submission ───────────────────────────────────────────────────
    remitForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        setLoading(true);

        const formData = new FormData(remitForm);
        const requestBody = Object.fromEntries(formData.entries());
        
        // Add defaults
        requestBody.target_currency = "ETB";

        try {
            const response = await fetch('/api/remittance', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            });

            if (!response.ok) {
                const err = await response.json();
                throw new Error(err.message || 'Initiation failed');
            }

            const data = await response.json();
            
            // Trigger CyberSource Hosted Checkout
            submitToCyberSource(data.signed_fields);

        } catch (err) {
            alert(err.message);
            setLoading(false);
        }
    });

    function submitToCyberSource(signedData) {
        const form = document.getElementById('cybersource-form');
        form.action = signedData.checkout_url;
        form.innerHTML = '';

        const { checkout_url, ...fields } = signedData;
        Object.entries(fields).forEach(([name, value]) => {
            if (value !== null && value !== undefined) {
                const input = document.createElement('input');
                input.type = 'hidden';
                input.name = name;
                input.value = value;
                form.appendChild(input);
            }
        });

        form.submit();
    }

    function setLoading(isLoading) {
        submitBtn.disabled = isLoading;
        submitBtn.textContent = isLoading ? 'Processing...' : 'Continue to Payment';
    }

    // Initial load
    fetchRate();
});
