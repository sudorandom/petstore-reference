import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { PetService } from "../gen/pet/v1/pet_pb";

const defaultUrl =
  typeof window !== "undefined"
    ? window.location.protocol === "http:"
      ? "http://localhost:8080"
      : "https://localhost:8080"
    : "https://localhost:8080";

export const transport = createConnectTransport({
  baseUrl: defaultUrl,
  fetch: (input, init) => {
    // Include browser session credentials/cookies by default for upstream IAP / OAuth proxy
    return fetch(input, { ...init, credentials: "include" });
  },
});

export function getPetClient() {
  return createClient(PetService, transport);
}
