import { Section } from "@/api/media";
import { VideoShelf } from "@/components/VideoShelf";

// Videos: everything under movies/ on the server. The Anime tab is the same screen for anime/.
export default function Videos() {
  return <VideoShelf section={Section.Movies} />;
}
