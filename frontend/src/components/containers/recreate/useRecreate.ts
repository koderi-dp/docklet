import { useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { ContainerService } from "@bindings/services";
import { OperationStatus } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";

export const recreateInfoKey = (connectionId: string, id: string) => ["container-recreate-info", connectionId, id] as const;

/** The editable spec of a container; never cached, so the dialog always starts from the live container. */
export function useRecreateInfo(connectionId: string, containerId: string) {
  return useQuery({
    queryKey: recreateInfoKey(connectionId, containerId),
    queryFn: () => ContainerService.RecreateSpec(connectionId, containerId),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
}

/**
 * On the detail page: when the container named `name` was replaced by a recreate
 * operation, follow it to the new container ID.
 */
export function useFollowRecreatedContainer(connectionId: string, name: string | undefined) {
  const navigate = useNavigate();
  // The container vanishes from the list when it is replaced, so remember its last known name.
  const lastName = useRef(name);
  if (name) lastName.current = name;
  useOnOperationDone("container.recreate", connectionId, (op) => {
    const known = lastName.current;
    if (op.status !== OperationStatus.OpSucceeded || !known || op.resource !== known) return;
    ContainerService.List(connectionId)
      .then((list) => {
        const fresh = (list ?? []).find((c) => c.name === known);
        if (fresh) navigate(`/containers/${fresh.id}`, { replace: true });
      })
      .catch(() => undefined);
  });
}
