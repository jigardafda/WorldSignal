import { Select } from "@mantine/core";
import { useCountries, countryLabel } from "../lib/countries";

interface Props {
  value: string | null;
  onChange: (value: string | null) => void;
  label?: string;
  placeholder?: string;
  required?: boolean;
  clearable?: boolean;
  error?: React.ReactNode;
  "data-testid"?: string;
}

/** A searchable country dropdown backed by the server's ISO 3166 reference data.
 * The value is the ISO alpha-2 code; options show the full name with flag. */
export function CountrySelect({ value, onChange, label, placeholder, required, clearable = true, error, ...rest }: Props) {
  const { list, loading } = useCountries();
  const data = list.map((c) => ({ value: c.code, label: countryLabel(c) }));
  return (
    <Select
      label={label}
      placeholder={loading ? "Loading countries…" : (placeholder ?? "Select a country")}
      data={data}
      value={value}
      onChange={onChange}
      searchable
      clearable={clearable}
      required={required}
      error={error}
      nothingFoundMessage="No matching country"
      data-testid={rest["data-testid"]}
    />
  );
}
