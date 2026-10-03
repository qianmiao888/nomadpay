import { BrowserProvider, Contract, Interface, JsonRpcProvider, hexlify, isAddress, parseUnits, randomBytes } from 'ethers';

const $ = id => document.getElementById(id);
const RPC = 'https://testnet-rpc.monad.xyz';
const CHAIN_ID = 10143n;
const CONTRACT = '0x0ab5ED99aA3fB5cfF20cF91bCcE50A4856150958';
const EVENT = 'event InvoicePaid(bytes32 indexed invoiceId, address indexed payer, address indexed recipient, uint256 amount)';
const ABI = ['function pay(bytes32 invoiceId, address recipient) payable', EVENT];
const TOPIC = '0x787fcac3b50ab9534e1ef2e289dfa2a75eb9481de9f9061d4773232074751582';
const EXPLORER = 'https://testnet.monadscan.com';
const SAMPLE = { id: '0xee5b7daea6d5de535d700d352b5421bca1f477b8729274f89e23f67be848d6c0', recipient: '0x05CE8a74b15B24087a025726aBEdC29da075AA32', amountMon: '0.001', description: 'Verified end-to-end test payment', fromBlock: 66910792, txHash: '0x4fbb21c39d7b3bdb49e71de8b572d923865b060608cc8928a55382e5c2d305c0' };
const rpc = new JsonRpcProvider(RPC, Number(CHAIN_ID));
const iface = new Interface(ABI);
let invoice;
let checking = false;

function error(message) { $('notice').textContent = message; $('notice').hidden = false; }
function clearError() { $('notice').hidden = true; }
function encode(value) { return btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(value)))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, ''); }
function decode(value) { return JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(value.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)))); }
function valid(value) {
  if (!value || !/^0x[0-9a-fA-F]{64}$/.test(value.id) || !isAddress(value.recipient) || typeof value.description !== 'string' || !value.description.trim() || value.description.length > 140 || !Number.isSafeInteger(value.fromBlock) || value.fromBlock < 66910792) throw new Error('Invalid invoice link.');
  const amount = parseUnits(value.amountMon, 18);
  if (amount <= 0n) throw new Error('Invalid invoice amount.');
  return { ...value, amountWei: amount };
}
function show() {
  $('create-view').hidden = true; $('invoice-view').hidden = false;
  $('view-description').textContent = invoice.description;
  $('view-amount').textContent = `${invoice.amountMon} MON`;
  $('view-recipient').textContent = invoice.recipient;
  $('status').textContent = invoice.paid ? 'PAID' : 'PENDING';
  $('status').className = `status ${invoice.paid ? 'paid' : ''}`;
  $('pay-button').hidden = !!invoice.paid;
  if (invoice.txHash) {
    const link = document.createElement('a');
    link.href = `${EXPLORER}/tx/${invoice.txHash}`; link.target = '_blank'; link.rel = 'noopener noreferrer';
    link.textContent = 'View verified transaction ↗'; $('tx-line').replaceChildren(link);
  } else $('tx-line').textContent = 'Checking Monad Testnet for a matching contract payment…';
}
async function checkPayment() {
  if (!invoice || checking || invoice.paid) return;
  checking = true;
  try {
    if (invoice.txHash) {
      const receipt = await rpc.getTransactionReceipt(invoice.txHash);
      if (receipt?.status === 1) {
        const match = receipt.logs.find(log => matches(log));
        if (match) { invoice.paid = true; show(); return; }
      }
    }
    const latest = await rpc.getBlockNumber();
    // Public RPCs cap log ranges. Search recent invoice blocks in bounded requests.
    for (let from = invoice.fromBlock; from <= latest; from += 1000) {
      const logs = await rpc.getLogs({ address: CONTRACT, topics: [TOPIC, invoice.id], fromBlock: from, toBlock: Math.min(from + 999, latest) });
      const match = logs.find(log => matches(log));
      if (match) { invoice.txHash = match.transactionHash; invoice.paid = true; show(); return; }
    }
    $('tx-line').textContent = 'No matching confirmed payment yet. Status checks automatically.';
  } catch (e) { error(`Could not check testnet payment: ${e.shortMessage || e.message}`); }
  finally { checking = false; }
}
function matches(log) {
  try {
    if (log.address.toLowerCase() !== CONTRACT.toLowerCase()) return false;
    const event = iface.parseLog(log);
    return event?.name === 'InvoicePaid' && event.args.invoiceId.toLowerCase() === invoice.id.toLowerCase() && event.args.recipient.toLowerCase() === invoice.recipient.toLowerCase() && event.args.amount === invoice.amountWei;
  } catch { return false; }
}

$('invoice-form').addEventListener('submit', async event => {
  event.preventDefault(); clearError();
  const values = Object.fromEntries(new FormData(event.currentTarget));
  try {
    if (!isAddress(values.recipient)) throw new Error('Enter a valid EVM wallet address.');
    const amount = parseUnits(values.amountMon, 18);
    if (amount <= 0n || !/^[0-9]+(\.[0-9]{1,18})?$/.test(values.amountMon)) throw new Error('Enter a positive MON amount with at most 18 decimals.');
    const fromBlock = await rpc.getBlockNumber();
    invoice = valid({ id: hexlify(randomBytes(32)), recipient: values.recipient, amountMon: values.amountMon, description: values.description.trim(), fromBlock });
    history.pushState({}, '', `?i=${encode({ id: invoice.id, recipient: invoice.recipient, amountMon: invoice.amountMon, description: invoice.description, fromBlock })}`);
    show(); checkPayment();
  } catch (e) { error(e.shortMessage || e.message); }
});
$('copy-button').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(location.href); $('copy-button').textContent = 'Copied!'; }
  catch { error('Could not copy the invoice link.'); }
});
$('pay-button').addEventListener('click', async () => {
  clearError();
  if (!window.ethereum) { error('Install an EVM wallet to pay.'); return; }
  const button = $('pay-button'); button.disabled = true;
  try {
    const wallet = new BrowserProvider(window.ethereum);
    await wallet.send('eth_requestAccounts', []);
    if ((await wallet.getNetwork()).chainId !== CHAIN_ID) await wallet.send('wallet_switchEthereumChain', [{ chainId: `0x${CHAIN_ID.toString(16)}` }]);
    const contract = new Contract(CONTRACT, ABI, await wallet.getSigner());
    const tx = await contract.pay(invoice.id, invoice.recipient, { value: invoice.amountWei });
    invoice.txHash = tx.hash; show();
    await tx.wait(); await checkPayment();
  } catch (e) { error(e.shortMessage || e.message); }
  finally { button.disabled = false; }
});

try {
  const query = new URLSearchParams(location.search);
  if (query.get('demo') === 'paid') invoice = valid(SAMPLE);
  else if (query.has('i')) invoice = valid(decode(query.get('i')));
  if (invoice) { show(); checkPayment(); setInterval(checkPayment, 15000); }
} catch (e) { error(e.shortMessage || e.message); }
