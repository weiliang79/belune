import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createToken, deleteToken, listTokens } from "@/lib/api/tokens";
import { queryKeys } from "./query-keys";

export function useTokens() {
  return useQuery({
    queryKey: queryKeys.tokens,
    queryFn: listTokens,
  });
}

export function useCreateToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      expiresInDays,
    }: {
      name: string;
      expiresInDays?: number;
    }) => createToken(name, expiresInDays),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens });
    },
  });
}

export function useDeleteToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteToken(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens });
    },
  });
}
