import { Section } from "@/api/media";
import { VideoShelf } from "@/components/VideoShelf";

// Anime: everything under anime/ on the server. Same screen as Videos, its own shelf.
export default function Anime() {
  return <VideoShelf section={Section.Anime} />;
}
