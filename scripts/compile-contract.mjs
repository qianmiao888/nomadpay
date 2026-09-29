import fs from 'node:fs';
import solc from 'solc';

const source = fs.readFileSync('contracts/NomadPay.sol', 'utf8');
const input = { language: 'Solidity', sources: { 'NomadPay.sol': { content: source } }, settings: { outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object'] } } } };
const output = JSON.parse(solc.compile(JSON.stringify(input)));
const errors = output.errors?.filter(e => e.severity === 'error') ?? [];
if (errors.length) throw new Error(errors.map(e => e.formattedMessage).join('\n'));
const contract = output.contracts['NomadPay.sol'].NomadPay;
fs.mkdirSync('artifacts', { recursive: true });
fs.writeFileSync('artifacts/NomadPay.json', JSON.stringify({ abi: contract.abi, bytecode: `0x${contract.evm.bytecode.object}` }, null, 2));
console.log('Compiled artifacts/NomadPay.json');
