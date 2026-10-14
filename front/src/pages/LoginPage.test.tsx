import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import LoginPage from "./LoginPage";
import { SessionProvider } from "../lib/session";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function jsonResponse(
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

function renderLogin() {
  return render(
    <SessionProvider>
      <LoginPage />
    </SessionProvider>,
  );
}

function submitCode(code: string) {
  fireEvent.change(screen.getByLabelText("Code"), { target: { value: code } });
  fireEvent.click(screen.getByRole("button", { name: "Log in" }));
}

// Probe answers guest so LoginPage mounts the way the guard leaves it.
function stubProbe() {
  return vi.fn().mockResolvedValueOnce(
    jsonResponse(401, {
      error: { code: "unauthenticated", message: "nope" },
    }),
  );
}

describe("LoginPage", () => {
  it("shows a client-side required error and skips the network on empty code", async () => {
    const fetchMock = stubProbe();
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    // The alert slot always exists (empty) so the card never resizes.
    expect(screen.getByRole("alert").textContent).toBe("");
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Log in" }));
    });

    expect(
      screen.getByText("Enter the 6-digit code or a recovery code."),
    ).toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("submits the code and leaves the form in a logged-in world", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(200, { ok: true }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(fetchMock).toHaveBeenLastCalledWith("/api/auth/login", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: "123456" }),
    });
    expect(screen.getByRole("alert").textContent).toBe("");
  });

  it("shows the invalid-code message on 401", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(401, {
        error: { code: "unauthenticated", message: "invalid code" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("000000");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Invalid code. Try again.",
    );
  });

  it("shows the rate-limit countdown from Retry-After on 429", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(
        429,
        { error: { code: "rate_limited", message: "slow down" } },
        { "Retry-After": "42" },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Too many attempts. Try again in 42s.",
    );
  });

  it("falls back to a generic rate-limit message without Retry-After", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(429, {
        error: { code: "rate_limited", message: "slow down" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Too many attempts. Try again later.",
    );
    expect(screen.getByRole("button", { name: "Log in" })).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("clears the code input when submitted", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(200, { ok: true }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByLabelText("Code")).toHaveProperty("value", "");
  });

  it("locks the form on 429 and counts down the retry window live", async () => {
    vi.useFakeTimers();
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(
        429,
        { error: { code: "rate_limited", message: "slow down" } },
        { "Retry-After": "2" },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await act(async () => {});

    await act(async () => {
      submitCode("123456");
    });

    const alert = () => screen.getByRole("alert").textContent;
    expect(alert()).toBe("Too many attempts. Try again in 2s.");
    expect(screen.getByLabelText("Code")).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: "Log in" })).toHaveProperty(
      "disabled",
      true,
    );

    // Enter-key resubmits are ignored for the whole lock window.
    fireEvent.submit(
      screen.getByRole("button", { name: "Log in" }).closest("form")!,
    );
    expect(alert()).toBe("Too many attempts. Try again in 2s.");
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(alert()).toBe("Too many attempts. Try again in 1s.");

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByRole("alert").textContent).toBe("");
    expect(screen.getByLabelText("Code")).toHaveProperty("disabled", false);
    expect(screen.getByRole("button", { name: "Log in" })).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("does not lock the form when Retry-After is not a usable duration", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(
        429,
        { error: { code: "rate_limited", message: "slow down" } },
        { "Retry-After": "0" },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Too many attempts. Try again later.",
    );
    expect(screen.getByLabelText("Code")).toHaveProperty("disabled", false);
  });

  it("stops the countdown timer when unmounted", async () => {
    vi.useFakeTimers();
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(
        429,
        { error: { code: "rate_limited", message: "slow down" } },
        { "Retry-After": "42" },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const view = renderLogin();
    await act(async () => {});

    await act(async () => {
      submitCode("123456");
    });
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    view.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("shows a network message when the login call fails", async () => {
    const fetchMock = stubProbe().mockRejectedValueOnce(
      new TypeError("offline"),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Could not reach the server. Check the connection and retry.",
    );
  });

  it("shows a generic message on unexpected server errors", async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(500, { error: { code: "internal_error", message: "boom" } }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Login failed. Try again.",
    );
  });

  it("shows a generic message when the failure is not an ApiError", async () => {
    // 200 with a non-JSON body: api() rethrows the raw parse error, which
    // loginErrorMessage must not mistake for API data.
    const fetchMock = stubProbe().mockResolvedValueOnce(
      new Response("not json", { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    expect(screen.getByRole("alert").textContent).toBe(
      "Login failed. Try again.",
    );
  });

  it("disables the button while the login call is in flight", async () => {
    let resolveLogin!: (r: Response) => void;
    const fetchMock = stubProbe().mockImplementationOnce(
      () =>
        new Promise<Response>((res) => {
          resolveLogin = res;
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderLogin();
    await waitFor(() => expect(screen.getByLabelText("Code")).toBeDefined());

    await act(async () => {
      submitCode("123456");
    });

    const button = screen.getByRole("button", { name: "Checking…" });
    expect(button).toHaveProperty("disabled", true);

    // A second submit while one is in flight is a no-op.
    fireEvent.submit(button.closest("form")!);
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => {
      resolveLogin(jsonResponse(200, { ok: true }));
    });
  });
});
