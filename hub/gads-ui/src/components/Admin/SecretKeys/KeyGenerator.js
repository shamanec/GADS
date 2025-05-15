import { Button } from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';

export default function KeyGenerator({ onGenerated }) {
  // Generate a secure random key with at least 32 bytes (encoded in base64)
  const generateSecureKey = () => {
    // Create a random array of 32 bytes
    const randomBytes = new Uint8Array(32);
    window.crypto.getRandomValues(randomBytes);
    
    // Convert to base64
    const base64Key = btoa(String.fromCharCode.apply(null, randomBytes));
    
    if (onGenerated) {
      onGenerated(base64Key);
    }
    
    return base64Key;
  };

  return (
    <Button
      variant="outlined"
      startIcon={<RefreshIcon />}
      onClick={generateSecureKey}
      size="small"
      sx={{ mt: 1 }}
    >
      Generate Secure Key
    </Button>
  );
} 