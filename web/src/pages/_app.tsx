import { ChakraProvider } from "@chakra-ui/react";
import React from "react";
import { CognitoProvider } from "../utils/context/cognitoContext";
import { TargetsProvider } from "../utils/context/targetsContext";
import { UserProvider } from "../utils/context/userContext";
import ErrorBoundary from "../utils/errorBoundary";
import { theme } from "../utils/theme";

export default function App({ children }: { children: React.ReactNode }) {
  return (
    <ChakraProvider theme={theme}>
      <ErrorBoundary>
        <CognitoProvider>
          <UserProvider>
            <TargetsProvider>{children}</TargetsProvider>
          </UserProvider>
        </CognitoProvider>
      </ErrorBoundary>
    </ChakraProvider>
  );
}
