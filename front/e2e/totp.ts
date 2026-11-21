import { createHmac } from "node:crypto";

// The E2E stack is enrolled (host-side, `server enroll --secret`) with this
// throwaway secret, so the browser can mint the same codes an
// authenticator would. Test-only: it lives in a disposable volume that
// `make e2e` deletes, and a real deployment never enrolls this value.
const DEFAULT_SECRET = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP";

const BASE32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
const STEP_SECONDS = 30;
const DIGITS = 6;

export function e2eTotpSecret(): string {
  return process.env.E2E_TOTP_SECRET ?? DEFAULT_SECRET;
}

// RFC4648 base32 -> bytes (padding and case tolerated).
export function base32Decode(input: string): Buffer {
  const clean = input.toUpperCase().replace(/=|\s/g, "");
  const bytes: number[] = [];
  let bits = 0;
  let value = 0;

  for (const symbol of clean) {
    const index = BASE32.indexOf(symbol);
    if (index === -1) throw new Error(`invalid base32 symbol: ${symbol}`);
    value = (value << 5) | index;
    bits += 5;
    if (bits >= 8) {
      bits -= 8;
      bytes.push((value >>> bits) & 0xff);
    }
  }

  return Buffer.from(bytes);
}

// RFC6238 TOTP: SHA1 over the 30s counter, 6 digits.
export function currentTotp(
  secret: string = e2eTotpSecret(),
  atMs: number = Date.now(),
): string {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(atMs / 1000 / STEP_SECONDS)));

  const digest = createHmac("sha1", base32Decode(secret))
    .update(counter)
    .digest();
  const offset = digest[digest.length - 1] & 0x0f;
  const binary =
    ((digest[offset] & 0x7f) << 24) |
    (digest[offset + 1] << 16) |
    (digest[offset + 2] << 8) |
    digest[offset + 3];

  return String(binary % 10 ** DIGITS).padStart(DIGITS, "0");
}
