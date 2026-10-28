import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useFetch } from "./useFetch";
import { api } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

vi.mock("./api", () => ({
  api: vi.fn(),
  isApiError: (err: unknown) =>
    typeof err === "object" && err !== null && "status" in err,
}));

describe("useFetch", () => {
  it("starts loading and lands data from the server", async () => {
    vi.mocked(api).mockResolvedValue({ data: [1, 2] });

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
    vi.mocked(api).mockRejectedValue({
      status: 400,
      code: "validation_error",
      message: "bad input",
    });

    const { result } = renderHook(() => useFetch("/things", "failed"));

    await waitFor(() => expect(result.current.error).toBe("bad input"));
    expect(result.current.loading).toBe(false);
  });

  it("falls back when the failure is not an ApiError", async () => {
    vi.mocked(api).mockRejectedValue(new SyntaxError("parse failed"));

    const { result } = renderHook(() => useFetch("/things", "failed"));

    await waitFor(() => expect(result.current.error).toBe("failed"));
  });

  it("ignores a fetch that settles after unmount", async () => {
    let resolve!: (data: unknown) => void;
    vi.mocked(api).mockImplementation(
      () =>
        new Promise((res) => {
          resolve = res;
        }),
    );

    const { unmount } = renderHook(() => useFetch("/things", "failed"));
    unmount();

    await act(async () => {
      resolve({ ok: true });
    });
    expect(vi.mocked(api)).toHaveBeenCalledTimes(1);
  });

  it("silently ignores AbortError when fetch is cancelled", async () => {
    vi.mocked(api).mockImplementation(
      () =>
        new Promise((_, reject) => {
          reject(new DOMException("Aborted", "AbortError"));
        }),
    );

    const { unmount } = renderHook(() => useFetch("/things", "failed"));
    unmount();

    await act(async () => {
      await Promise.resolve();
    });

    expect(vi.mocked(api)).toHaveBeenCalledTimes(1);
  });

  it("does not flip loading flag for an aborted fetch", async () => {
    vi.mocked(api).mockImplementation(
      () =>
        new Promise((_, reject) => {
          reject(new DOMException("Aborted", "AbortError"));
        }),
    );

    const { result, unmount } = renderHook(() => useFetch("/things", "failed"));
    expect(result.current.loading).toBe(true);

    unmount();

    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.data).toBeNull();
  });

  it("stays silent when an aborted fetch rejects as a network error", async () => {
    // The transport converts most fetch failures into a network_error
    // ApiError, so the hook must key cancellation off its own signal —
    // not the error's shape — or a week-slide abort surfaces as a load
    // error (the calendar-only regression).
    let callCount = 0;
    let resolveSecond!: (value: unknown) => void;
    vi.mocked(api).mockImplementation((_path, options) => {
      callCount += 1;
      if (callCount === 1) {
        return new Promise((_, reject) => {
          options?.signal?.addEventListener("abort", () =>
            reject({
              status: 0,
              code: "network_error",
              message: "network request failed",
            }),
          );
        });
      }
      return new Promise((resolve) => {
        resolveSecond = resolve;
      });
    });

    const { result, rerender } = renderHook(
      ({ path }: { path: string }) =>
        useFetch<{ data: string[] }>(path, "failed"),
      { initialProps: { path: "/first" } },
    );

    // Path change cleans up the first effect: its fetch aborts and rejects
    // with the converted shape while the replacement is still in flight.
    rerender({ path: "/second" });

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.error).toBeNull();
    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolveSecond({ data: ["b"] });
    });
    expect(result.current.data).toEqual({ data: ["b"] });
    expect(result.current.error).toBeNull();
    expect(result.current.loading).toBe(false);
  });

  it("refetches without flipping the loading flag and keeps previous data", async () => {
    let listCalls = 0;
    vi.mocked(api).mockImplementation(() => {
      listCalls += 1;
      return Promise.resolve({ data: [listCalls] });
    });

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
    vi.mocked(api)
      .mockRejectedValueOnce({
        status: 400,
        code: "validation_error",
        message: "boom",
      })
      .mockResolvedValue({ ok: true });

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
    vi.mocked(api)
      .mockRejectedValueOnce({
        status: 0,
        code: "network_error",
        message: "network request failed",
      })
      .mockResolvedValue({ ok: true });

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

  it("keeps previous data and clears loading on background fetch failure", async () => {
    let callCount = 0;
    vi.mocked(api).mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        return Promise.resolve({ data: [1] });
      }
      return Promise.reject({
        status: 400,
        code: "validation_error",
        message: "boom",
      });
    });

    const { result } = renderHook(() =>
      useFetch<{ data: number[] }>("/things", "failed"),
    );
    await waitFor(() => expect(result.current.data).toEqual({ data: [1] }));
    expect(result.current.loading).toBe(false);

    act(() => {
      result.current.refetch();
    });

    await waitFor(() => expect(result.current.error).toBe("boom"));
    expect(result.current.loading).toBe(false);
    expect(result.current.data).toEqual({ data: [1] });
  });

  it("keeps previous data and clears loading on background network error", async () => {
    let callCount = 0;
    vi.mocked(api).mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        return Promise.resolve({ data: [1] });
      }
      return Promise.reject({
        status: 0,
        code: "network_error",
        message: "network request failed",
      });
    });

    const { result } = renderHook(() =>
      useFetch<{ data: number[] }>("/things", "failed"),
    );
    await waitFor(() => expect(result.current.data).toEqual({ data: [1] }));
    expect(result.current.loading).toBe(false);

    act(() => {
      result.current.refetch();
    });

    await waitFor(() =>
      expect(result.current.error).toBe("network request failed"),
    );
    expect(result.current.loading).toBe(false);
    expect(result.current.data).toEqual({ data: [1] });
  });

  it("keeps previous data and clears the error when the path changes", async () => {
    vi.mocked(api).mockImplementation((url: string) =>
      url.includes("second")
        ? Promise.resolve({ data: ["b"] })
        : Promise.reject({
            status: 400,
            code: "validation_error",
            message: "boom",
          }),
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

  it("keeps previous data visible when a path change fails", async () => {
    let callCount = 0;
    vi.mocked(api).mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        return Promise.resolve({ data: ["a"] });
      }
      return Promise.reject({
        status: 0,
        code: "network_error",
        message: "network request failed",
      });
    });

    const { result, rerender } = renderHook(
      ({ path }: { path: string }) =>
        useFetch<{ data: string[] }>(path, "failed"),
      { initialProps: { path: "/first" } },
    );
    await waitFor(() => expect(result.current.data).toEqual({ data: ["a"] }));

    rerender({ path: "/second" });
    await waitFor(() =>
      expect(result.current.error).toBe("network request failed"),
    );
    expect(result.current.loading).toBe(false);
    expect(result.current.data).toEqual({ data: ["a"] });
  });

  it("re-runs the effect when fallbackError changes", async () => {
    vi.mocked(api).mockRejectedValue(new SyntaxError("parse failed"));

    const { result, rerender } = renderHook(
      ({ fallback }: { fallback: string }) => useFetch("/things", fallback),
      { initialProps: { fallback: "first failed" } },
    );
    await waitFor(() => expect(result.current.error).toBe("first failed"));

    rerender({ fallback: "second failed" });
    await waitFor(() => expect(vi.mocked(api)).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.error).toBe("second failed"));
  });
});
