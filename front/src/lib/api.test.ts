import { afterEach, describe, expect, it, vi } from "vitest";
import { api, isApiError, type ApiError } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("api", () => {
  it("GETs with credentials and parses the JSON body", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(api("/session")).resolves.toEqual({ ok: true });

    expect(fetchMock).toHaveBeenCalledWith("/api/session", {
      method: "GET",
      credentials: "include",
      headers: {},
      body: undefined,
    });
  });

  it("sends a JSON body and content-type for mutations", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "n1" }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      api("/notes", { method: "POST", body: { title: "Hi" } }),
    ).resolves.toEqual({ id: "n1" });

    expect(fetchMock).toHaveBeenCalledWith("/api/notes", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: "Hi" }),
    });
  });

  it("returns undefined for 204 responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(null, { status: 204 })),
    );

    await expect(
      api("/auth/logout", { method: "POST" }),
    ).resolves.toBeUndefined();
  });

  it("throws ApiError with code, message and fields on 400", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: {
            code: "validation_error",
            message: "bad",
            fields: { title: "required" },
          },
        }),
      ),
    );

    await expect(
      api("/notes", { method: "POST", body: {} }),
    ).rejects.toMatchObject({
      status: 400,
      code: "validation_error",
      message: "bad",
      fields: { title: "required" },
    });
  });

  it("throws ApiError on 401 without redirecting", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(401, {
          error: { code: "unauthenticated", message: "nope" },
        }),
      ),
    );
    const before = window.location.pathname;

    await expect(api("/session")).rejects.toMatchObject({ status: 401 });

    // Redirect policy belongs to the session guard, not lib/api (FRONTEND.md: lib stays router-free).
    expect(window.location.pathname).toBe(before);
  });

  it("throws a generic ApiError when the error body is not JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(new Response("<html>502</html>", { status: 502 })),
    );

    const err = await api("/session").catch((e: unknown) => e);
    expect(err).toMatchObject({
      status: 502,
      code: "error",
      message: "request failed",
    });
    expect((err as ApiError).fields).toBeUndefined();
  });

  it("falls back to generic code/message when the envelope has no error fields", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: {} }), {
          status: 500,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const err = await api("/session").catch((e: unknown) => e);
    expect(err).toMatchObject({
      status: 500,
      code: "error",
      message: "request failed",
    });
    expect((err as ApiError).fields).toBeUndefined();
  });

  it("falls back to generic code/message when the JSON body has no error key", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(503, { message: "oops" })),
    );

    await expect(api("/session")).rejects.toMatchObject({
      status: 503,
      code: "error",
      message: "request failed",
    });
  });

  it("captures Retry-After as seconds on 429", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            error: { code: "rate_limited", message: "slow down" },
          }),
          {
            status: 429,
            headers: {
              "Content-Type": "application/json",
              "Retry-After": "42",
            },
          },
        ),
      ),
    );

    await expect(
      api("/auth/login", { method: "POST", body: { code: "1" } }),
    ).rejects.toMatchObject({ retryAfter: 42, code: "rate_limited" });
  });

  it("leaves retryAfter unset when the 429 carries no Retry-After", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(429, {
          error: { code: "rate_limited", message: "slow down" },
        }),
      ),
    );

    const err = await api("/auth/login", {
      method: "POST",
      body: { code: "1" },
    }).catch((e: unknown) => e);
    expect(err).toMatchObject({ code: "rate_limited" });
    expect((err as ApiError).retryAfter).toBeUndefined();
  });

  it("rethrows network failures as ApiError with status 0", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("network down")),
    );

    await expect(api("/session")).rejects.toMatchObject({
      status: 0,
      code: "network_error",
    });
  });
});

describe("isApiError", () => {
  it("accepts thrown API data and rejects anything else", () => {
    expect(
      isApiError({ status: 401, code: "unauthenticated", message: "nope" }),
    ).toBe(true);
    expect(isApiError(new TypeError("network down"))).toBe(false);
    expect(isApiError(null)).toBe(false);
    expect(isApiError("boom")).toBe(false);
  });
});
