import { SymbolView, type AndroidSymbol, type SFSymbol } from "expo-symbols";
import type { ColorValue } from "react-native";

type Props = {
  sf: SFSymbol; // iOS: "play.fill"
  md: AndroidSymbol; // Material Symbol, Android and web: "play_arrow"
  size?: number;
  color?: ColorValue;
};

// Icon draws SF Symbols on iOS, so icons match the system's own glass controls in weight and
// size, and Material Symbols on Android and web, where SF Symbols don't exist.
export function Icon({ sf, md, size = 24, color }: Props) {
  return <SymbolView name={{ ios: sf, android: md, web: md }} size={size} tintColor={color} />;
}
