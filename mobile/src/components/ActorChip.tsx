import { router } from "expo-router";

import type { ActorTag } from "@/api/media";
import { Chip } from "./Chip";

// ActorChip opens the Library filtered to this actor. push, not navigate, so back returns
// here. The router encodes the name (spaces, accents, "&") for the URL; never concatenate it.
export function ActorChip({ tag }: { tag: ActorTag }) {
  return (
    <Chip label={tag.name} onPress={() => router.push({ pathname: "/library", params: { actor: tag.name } })} />
  );
}
