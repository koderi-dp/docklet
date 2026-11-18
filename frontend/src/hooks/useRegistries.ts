import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { create } from "zustand";
import { RegistryService } from "@bindings/services";
import type { RegistryEntry } from "@bindings/model";
import { toastError } from "@/lib/errors";

export const registriesKey = ["registries"] as const;

/** Registries the user is signed in to (host, user name, where the secret lives; never the secret). */
export function useRegistries() {
  return useQuery({ queryKey: registriesKey, queryFn: () => RegistryService.List(), retry: false, staleTime: 10_000 });
}

/** Refresh everything that depends on stored credentials. */
export function useInvalidateRegistries() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: registriesKey });
    void qc.invalidateQueries({ queryKey: ["registry-auth"] });
  };
}

export function useSignOut() {
  const invalidate = useInvalidateRegistries();
  return useMutation({
    mutationFn: (e: RegistryEntry) => RegistryService.SignOut(e.host, e.store),
    onSuccess: invalidate,
    onError: (e) => toastError("Could not sign out", e),
  });
}

interface SignInRequest {
  /** Registry host to preselect (docker.io, ghcr.io, host:port). */
  registry?: string;
  onSignedIn?: (entry: RegistryEntry) => void;
}

interface SignInState {
  request: SignInRequest | null;
  open: (r?: SignInRequest) => void;
  close: () => void;
}

/** The single sign-in dialog (mounted once in the top bar) is opened from Settings, push/pull and failed operations. */
export const useSignInDialog = create<SignInState>((set) => ({
  request: null,
  open: (r = {}) => set({ request: r }),
  close: () => set({ request: null }),
}));
