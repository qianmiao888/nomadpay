import { BrowserProvider, ContractFactory, Interface } from 'ethers';
import artifact from '../artifacts/NomadPay.json';

const button = document.getElementById('deploy-button');
const notice = document.getElementById('notice');
const result = document.getElementById('result');
const settings = document.getElementById('settings');
const explorer = document.getElementById('explorer-link');

button.addEventListener('click', async () => {
  notice.hidden = true;
  if (!window.ethereum) {
    notice.textContent = 'Install a browser wallet such as MetaMask first.';
    notice.hidden = false;
    return;
  }
  button.disabled = true;
  try {
    const provider = new BrowserProvider(window.ethereum);
    await provider.send('eth_requestAccounts', []);
    const network = await provider.getNetwork();
    if (network.chainId !== 10143n) {
      throw new Error('Switch your wallet to Monad Testnet (chain ID 10143), then retry.');
    }
    const factory = new ContractFactory(artifact.abi, artifact.bytecode, await provider.getSigner());
    const contract = await factory.deploy();
    const receipt = await contract.deploymentTransaction().wait();
    if (!receipt || receipt.status !== 1) throw new Error('Deployment transaction failed.');
    const address = await contract.getAddress();
    const topic = new Interface(artifact.abi).getEvent('InvoicePaid').topicHash;
    settings.textContent = `CONTRACT_ADDRESS=${address}\nSTART_BLOCK=${receipt.blockNumber}\nEVENT_TOPIC=${topic}`;
    explorer.href = `https://testnet.monadscan.com/address/${address}`;
    result.hidden = false;
    button.textContent = 'Deployed';
  } catch (error) {
    notice.textContent = error.shortMessage || error.message;
    notice.hidden = false;
    button.disabled = false;
  }
});
