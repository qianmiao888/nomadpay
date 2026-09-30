import { BrowserProvider, Contract, isAddress } from 'ethers';

const $ = id => document.getElementById(id);
const ABI = ['function pay(bytes32 invoiceId, address recipient) payable'];
let config;
let currentInvoice;
function error(message) { $('notice').textContent = message; $('notice').hidden = false; }
function clearError() { $('notice').hidden = true; }
async function api(path, options) {
  const response = await fetch(path, options);
  const value = await response.json();
  if (!response.ok) throw new Error(value.error || `Request failed: ${response.status}`);
  return value;
}
function short(address) { return `${address.slice(0, 8)}…${address.slice(-6)}`; }
function display(invoice) {
  currentInvoice = invoice;
  $('create-view').hidden = true; $('invoice-view').hidden = false;
  $('view-description').textContent = invoice.description;
  $('view-amount').textContent = `${invoice.amountMon} MON`;
  $('view-recipient').textContent = invoice.recipient;
  $('status').textContent = invoice.status.toUpperCase();
  $('status').className = `status ${invoice.status}`;
  $('pay-button').hidden = invoice.status === 'paid';
  if (invoice.txHash) {
    const link = document.createElement('a');
    link.href = `${config.explorerUrl.replace(/\/$/, '')}/tx/${invoice.txHash}`;
    link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = `View transaction ${short(invoice.txHash)}`;
    $('tx-line').replaceChildren(link);
  } else $('tx-line').textContent = 'Payment status updates after onchain confirmation.';
}
async function refresh() {
  if (!currentInvoice) return;
  try { display(await api(`/api/invoices/${currentInvoice.id}`)); } catch (e) { error(e.message); }
}
async function init() {
  try { config = await api('/api/config'); } catch (e) { error(e.message); return; }
  const id = new URLSearchParams(location.search).get('invoice');
  if (id) { try { display(await api(`/api/invoices/${encodeURIComponent(id)}`)); } catch (e) { error(e.message); } }
  if (!config.paymentsEnabled) { $('pay-button').disabled = true; $('pay-button').textContent = 'Payments not configured'; }
  setInterval(refresh, 12000);
}
$('invoice-form').addEventListener('submit', async event => {
  event.preventDefault(); clearError();
  const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
  if (!isAddress(values.recipient)) { error('Enter a valid EVM wallet address.'); return; }
  const button = form.querySelector('button'); button.disabled = true;
  try {
    const invoice = await api('/api/invoices', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(values) });
    history.pushState({}, '', `?invoice=${invoice.id}`); display(invoice);
  } catch (e) { error(e.message); } finally { button.disabled = false; }
});
$('copy-button').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(location.href); $('copy-button').textContent = 'Copied!'; } catch { error('Could not copy link.'); }
});
$('pay-button').addEventListener('click', async () => {
  clearError();
  if (!window.ethereum) { error('Install an EVM wallet to pay.'); return; }
  const button = $('pay-button'); button.disabled = true;
  try {
    const provider = new BrowserProvider(window.ethereum);
    await provider.send('eth_requestAccounts', []);
    const network = await provider.getNetwork();
    if (network.chainId !== BigInt(config.chainId)) {
      await provider.send('wallet_switchEthereumChain', [{ chainId: `0x${Number(config.chainId).toString(16)}` }]);
    }
    const contract = new Contract(config.contractAddress, ABI, await provider.getSigner());
    const tx = await contract.pay(currentInvoice.id, currentInvoice.recipient, { value: BigInt(currentInvoice.amountWei) });
    $('tx-line').textContent = `Transaction submitted: ${short(tx.hash)}. Waiting for confirmations…`;
    await tx.wait(); await refresh();
  } catch (e) { error(e.shortMessage || e.message); } finally { button.disabled = false; }
});

function showInsights(report) {
  const box = $('insights-result'); box.replaceChildren(); box.hidden = false;
  const title = document.createElement('strong');
  title.textContent = report.synthetic ? 'Synthetic sample preview — not live payment data' : 'Your payment history';
  box.append(title);
  const summary = document.createElement('p');
  summary.textContent = report.status === 'ready'
    ? `${report.sampleCount} paid invoices analyzed. Median amount: ${report.medianAmountMon} MON. Median settlement delay: ${report.medianDelayHours} hours.`
    : `${report.sampleCount} paid invoices available; ${report.minimumSamples} are required before scoring.`;
  box.append(summary);
  const explanation = document.createElement('p'); explanation.textContent = report.explanation; box.append(explanation);
  if (report.anomalies.length) {
    const list = document.createElement('ul');
    for (const anomaly of report.anomalies) {
      const item = document.createElement('li');
      item.textContent = `${anomaly.invoiceId}: ${anomaly.amountMon} MON, ${anomaly.delayHours} hours, anomaly score ${anomaly.score}`;
      list.append(item);
    }
    box.append(list);
  } else if (report.status === 'ready') {
    const none = document.createElement('p'); none.textContent = 'No unusual payments were detected in this history.'; box.append(none);
  }
}
$('analyze-button').addEventListener('click', async () => {
  clearError();
  const recipient = currentInvoice?.recipient || $('recipient').value.trim();
  if (!isAddress(recipient)) { error('Enter a valid recipient wallet address first.'); return; }
  try { showInsights(await api(`/api/insights?recipient=${encodeURIComponent(recipient)}`)); }
  catch (e) { error(e.message); }
});
$('sample-button').addEventListener('click', async () => {
  clearError();
  try { showInsights(await api('/api/insights/demo')); }
  catch (e) { error(e.message); }
});
init();
