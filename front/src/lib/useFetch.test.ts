import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useFetch } from "./useFetch";

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("useFetch", () => {
  it("starts loading and lands data from the server", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(200, { data: [1, 2] })),
    );

    const { result } = renderHook(() =>
      useFetch<{ data: number[] }>("/things", "failed"),
    );

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual({ data: [1, 2] });
    expect(result.current.error).toBeNull();
  });

  it("surfaces the server message on an ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: { code: "validation_error", message: "bad input" },
        }),
      ),
    );

    const { result } = renderHook(() => useFetch("/things", "failed"));

    await waitFor(() => expect(result.current.error).toBe("bad input"));
    expect(result.current.loading).toBe(false);
  });

  it("falls back when the failure is not an ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(new Response("<html>nope</html>", { status: 200 })),
    );

    const { result } = renderHook(() => useFetch("/things", "failed"));

    await waitFor(() => expect(result.current.error).toBe("failed"));
  });

  it("ignores a fetch that settles after unmount", async () => {
    let resolve!: (r: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(
        () =>
          new Promise<Response>((res) => {
            resolve = res;
          }),
      ),
    );

    const { unmount } = renderHook(() => useFetch("/things", "failed"));
    unmount();

    await act(async () => {
      resolve(jsonResponse(200, { ok: true }));
    });
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("refetches without flipping the loading flag and keeps previous data", async () => {
    let listCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => {
        listCalls += 1;
        return Promise.resolve(jsonResponse(200, { data: [listCalls] }));
      }),
    );

    const { result } = renderHook(() =>
      useFetch<{ data: number[] }>("/things", "failed"),
    );
    await waitFor(() => expect(result.current.data).toEqual({ data: [1] }));

    act(() => {
      result.current.refetch();
    });

    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    await waitFor(() => expect(result.current.data).toEqual({ data: [2] }));
    expect(result.current.loading).toBe(false);
  });

  it("clears a stale error when refetching", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(
          jsonResponse(400, {
            error: { code: "validation_error", message: "boom" },
          }),
        )
        .mockResolvedValue(jsonResponse(200, { ok: true })),
    );

    const { result } = renderHook(() =>
      useFetch<{ ok: boolean }>("/things", "failed"),
    );
    await waitFor(() => expect(result.current.error).toBe("boom"));

    act(() => {
      result.current.refetch();
    });
    expect(result.current.error).toBeNull();
    await waitFor(() => expect(result.current.data).toEqual({ ok: true }));
  });

  it("shows the skeleton again on retry", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockRejectedValueOnce(new TypeError("offline"))
        .mockResolvedValue(jsonResponse(200, { ok: true })),
    );

    const { result } = renderHook(() =>
      useFetch<{ ok: boolean }>("/things", "failed"),
    );
    await waitFor(() =>
      expect(result.current.error).toBe("network request failed"),
    );

    act(() => {
      result.current.retry();
    });
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.data).toEqual({ ok: true }));
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("keeps previous data and clears the error when the path changes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) =>
        Promise.resolve(
          url.includes("second")
            ? jsonResponse(200, { data: ["b"] })
            : jsonResponse(400, {
                error: { code: "validation_error", message: "boom" },
              }),
        ),
      ),
    );

    const { result, rerender } = renderHook(
      ({ path }: { path: string }) =>
        useFetch<{ data: string[] }>(path, "failed"),
      { initialProps: { path: "/first" } },
    );
    await waitFor(() => expect(result.current.error).toBe("boom"));

    rerender({ path: "/second" });
    expect(result.current.error).toBeNull();
    await waitFor(() => expect(result.current.data).toEqual({ data: ["b"] }));
  });

  it("re-runs the effect when fallbackError changes", async () => {
    // A 200 with a non-JSON body throws a SyntaxError, not an ApiError:
    // the only way to reach the fallback message.
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response("<html>nope</html>", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const { result, rerender } = renderHook(
      ({ fallback }: { fallback: string }) => useFetch("/things", fallback),
      { initialProps: { fallback: "first failed" } },
    );
    await waitFor(() => expect(result.current.error).toBe("first failed"));

    rerender({ fallback: "second failed" });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.error).toBe("second failed"));
  });
});
